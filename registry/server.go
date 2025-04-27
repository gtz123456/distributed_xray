package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-distributed/registry/heartbeat"
	"go-distributed/utils"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type registry struct {
	registrationsMap map[ServiceName][]Registration
	heartbeatServer  *heartbeat.HeartBeatServer
	mutex            *sync.RWMutex
}

func (r *registry) add(reg Registration) error {
	// Check and remove duplicate registrations
	r.mutex.Lock()
	exists := false
	for _, existingReg := range r.registrationsMap[reg.ServiceName] {
		if existingReg.ServiceURL == reg.ServiceURL {
			exists = true
			break
		}
	}
	if !exists {
		r.registrationsMap[reg.ServiceName] = append(r.registrationsMap[reg.ServiceName], reg)
	}
	r.mutex.Unlock()
	err := r.sendRequiredServices(reg)
	r.notify(patch{
		Added: []patchEntry{
			{
				Name: reg.ServiceName,
				URL:  reg.ServiceURL,
			},
		},
	})
	return err
}

func (r *registry) remove(serviceName ServiceName, url string) error {
	for i := range r.registrationsMap[serviceName] {
		if string(r.registrationsMap[serviceName][i].ServiceURL) == url {
			r.notify(patch{
				Removed: []patchEntry{
					{
						Name: r.registrationsMap[serviceName][i].ServiceName,
						URL:  r.registrationsMap[serviceName][i].ServiceURL,
					},
				},
			})
			r.mutex.Lock()
			r.registrationsMap[serviceName] = append(r.registrationsMap[serviceName][:i], r.registrationsMap[serviceName][i+1:]...)
			r.mutex.Unlock()
			fmt.Println("Removed service at URL: ", url)
			return nil
		}
	}
	return fmt.Errorf("service at URL %s not found", url)
}

func (r registry) notify(fullPatch patch) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, regs := range r.registrationsMap {
		for _, reg := range regs {
			go func(reg Registration) {
				for _, reqService := range reg.RequiredServices {
					p := patch{Added: []patchEntry{}, Removed: []patchEntry{}}
					sendUpdate := false
					for _, added := range fullPatch.Added {
						if added.Name == reqService {
							p.Added = append(p.Added, added)
							sendUpdate = true
						}
					}
					for _, removed := range fullPatch.Removed {
						if removed.Name == reqService {
							p.Removed = append(p.Removed, removed)
							sendUpdate = true
						}
					}
					if sendUpdate {
						err := r.sendPatch(reg.ServiceUpdateURL, p)
						if err != nil {
							log.Println(err)
							return
						}
					}
				}
			}(reg)
		}
	}
}

func (r registry) sendRequiredServices(reg Registration) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	var p patch

	// Create a patch with the current registrations for the required services
	for _, serviceName := range reg.RequiredServices {
		if services, ok := r.registrationsMap[serviceName]; ok {
			for _, service := range services {
				p.Added = append(p.Added, patchEntry{
					Name: service.ServiceName, // Use the Name for the entry
					URL:  service.ServiceURL,  // Use the ServiceURL for the entry
				})
			}
		}
	}

	if len(p.Added) == 0 && len(p.Removed) == 0 {
		log.Println("No services to send in the patch for", reg.ServiceName)
		return nil
	}

	err := r.sendPatch(reg.ServiceUpdateURL, p)
	if err != nil {
		return err
	}
	return nil
}

func (r registry) sendPatch(url string, p patch) error {
	d, err := json.Marshal(p)
	if err != nil {
		return err
	}

	// Send the patch to the service with regkey
	buf := bytes.NewBuffer(d)
	res, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return err
	}

	regkey := utils.Regkey()
	res.Header.Add("Content-Type", "application/json")
	res.Header.Add("regkey", regkey)

	log.Println("Sending patch to: ", url)
	resp, err := http.DefaultClient.Do(res)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to send patch. Registry service responded with status code %v", resp.StatusCode)
	}

	log.Println("Patch sent successfully")

	return nil
}

var reg = registry{
	registrationsMap: make(map[ServiceName][]Registration), // Initialize the map to store registrations
	mutex:            new(sync.RWMutex),
}

type RegistryService struct{}

func (s RegistryService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		serviceName := r.URL.Query().Get("serviceName")
		if serviceName == "" {
			// If no service name is provided, return error
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Service name is required"))
		}

		// Return the list of registrations for the requested service name
		reg.mutex.RLock()
		defer reg.mutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if services, ok := reg.registrationsMap[ServiceName(serviceName)]; ok {
			// Marshal the registrations to JSON and return
			d, err := json.Marshal(services)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(d)
		} else {
			// If no registrations found for the service name, return an empty array
			log.Printf("No registrations found for service name: %s", serviceName)
			w.Write([]byte("[]")) // Return an empty array if no registrations found
		}

		return

	case http.MethodPost:
		// Check if the node is authorized
		if utils.Regkey() != r.Header.Get("regkey") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Decode the request
		dec := json.NewDecoder(r.Body)
		var r Registration
		err := dec.Decode(&r)

		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if r.ServiceName == "" || r.ServiceURL == "" {
			log.Println("Service name or URL is empty")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		log.Printf("Adding service %s with URL: %s", r.ServiceName, r.ServiceURL)

		// generate uuid as ServiceID
		r.ServiceID = utils.GenerateUUID()

		// update last heartbeat for the service
		if r.ServiceID != "" {
			reg.heartbeatServer.Mutex.Lock()
			reg.heartbeatServer.LastHeartBeat[r.ServiceID] = time.Now()
			reg.heartbeatServer.Mutex.Unlock()
		}

		// Add the service to the registry
		err = reg.add(r)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.ServiceID))

	case http.MethodDelete:
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		url := string(payload)
		serviceName := r.URL.Query().Get("serviceName")
		if serviceName == "" {
			log.Println("Service name is missing in the query parameters")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Removing service %s at URL: %s", serviceName, url)
		err = reg.remove(ServiceName(serviceName), url)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (r *registry) IsServiceRegistered(serviceID string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, registrations := range r.registrationsMap {
		for _, registration := range registrations {
			if registration.ServiceID == serviceID {
				return true
			}
		}
	}
	return false
}

// remove inactive services from the registry
func removeInactiveServices() {
	var snapshot map[ServiceName][]Registration
	var lastHeartBeat map[string]time.Time

	reg.mutex.RLock()
	snapshot = make(map[ServiceName][]Registration)
	for k, v := range reg.registrationsMap {
		snapshot[k] = make([]Registration, len(v))
		copy(snapshot[k], v)
	}
	reg.mutex.RUnlock()

	reg.heartbeatServer.Mutex.RLock()
	lastHeartBeat = make(map[string]time.Time)
	for k, v := range reg.heartbeatServer.LastHeartBeat {
		lastHeartBeat[k] = v
	}
	reg.heartbeatServer.Mutex.RUnlock()

	for serviceName, registrations := range snapshot {
		for _, registration := range registrations {
			fmt.Println("Checking service: ", serviceName, " at URL: ", registration.ServiceURL, " last heartbeat: ", lastHeartBeat[registration.ServiceID])
			if time.Since(lastHeartBeat[registration.ServiceID]) > 20*time.Second {
				log.Printf("Removing inactive service %s at URL: %s", serviceName, registration.ServiceURL)
				if err := reg.remove(ServiceName(serviceName), registration.ServiceURL); err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func NewRegistryService(HBServer *heartbeat.HeartBeatServer) *RegistryService {
	reg.heartbeatServer = HBServer
	reg.heartbeatServer.Validator = &reg

	go func() {
		for range time.Tick(20 * time.Second) {
			log.Println("Checking inactive services...")
			removeInactiveServices()
		}
	}()
	return &RegistryService{}
}
