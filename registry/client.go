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
	"net/url"
	"sync"
	"time"
)

func RegisterRequest(r *Registration) error {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	err := enc.Encode(r)

	if err != nil {
		return err
	}

	res, err := http.NewRequest(http.MethodPost, ServerURL, buf)
	if err != nil {
		return err
	}

	regkey := utils.Regkey()
	res.Header.Add("Content-Type", "application/json")
	res.Header.Add("regkey", regkey)

	log.Println("Registering service at " + ServerURL)
	for {
		resp, err := http.DefaultClient.Do(res)
		if err == nil && resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			r.ServiceID = string(body)
			log.Printf("Service registered with ID: %s\n", r.ServiceID)
			break
		}
		log.Println("Failed to register service. Retry after 3 seconds...")
		time.Sleep(3 * time.Second)
	}

	return nil
}

func RegisterService(r *Registration) error {
	serviceUpdatedURL, err := url.Parse(r.ServiceUpdateURL)
	if err != nil {
		return err
	}

	log.Println("Service update URL Path: ", serviceUpdatedURL.Path)
	log.Println("Service URL: ", r.ServiceURL)
	http.Handle(serviceUpdatedURL.Path, &serviceUpdateHandler{})

	err = RegisterRequest(r)
	if err != nil {
		log.Println("Failed to register service: ", err)
	}

	interval := 3 * time.Second

	registryHeartbeatURL := "http://" + ServerIP + ":" + ServerPort + "/heartbeat/"

	hb := heartbeat.BasicHeartbeat{
		ServiceID: r.ServiceID,
		URL:       registryHeartbeatURL,
	}

	go func() {
		for {
			// log.Println("Sending heartbeat to registry service at " + registryHeartbeatURL)
			err = hb.SendHeartbeat()
			if err != nil {
				log.Printf("Failed to send heartbeat: %v\n", err)
				// register service again if returns 401 Unauthorized
				log.Println("error " + err.Error())
				if err.Error() == "Service not authorized" {
					time.Sleep(interval)
					log.Println("Re-registering service...")
					Prov.services = make(map[ServiceName][]Registration) // clear all cached providers
					err = RegisterRequest(r)
					if err != nil {
						log.Printf("Failed to re-register service: %v\n", err)
					}
					hb.ServiceID = r.ServiceID
				}
			}
			time.Sleep(interval)
			// log.Printf("Sent heartbeat to registry service at %s\n", registryHeartbeatURL)
		}
	}()

	return nil
}

type serviceUpdateHandler struct{}

func (s *serviceUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// only accept request from registry service
	if r.Header.Get("regkey") != utils.Regkey() {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	dec := json.NewDecoder(r.Body)
	var p patch
	err := dec.Decode(&p)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fmt.Println("Received patch: ", p)
	Prov.Update(p)
}

func ShutdownService(url string) error {
	req, err := http.NewRequest(http.MethodDelete, ServerURL, bytes.NewBuffer([]byte(url)))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "text/plain")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to deregister service. Registry service responded with status code %v", res.StatusCode)
	}

	return nil
}

type providers struct {
	services map[ServiceName][]Registration
	mutex    *sync.RWMutex
}

func (p *providers) Update(patch patch) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, reg := range patch.Added {
		if _, ok := p.services[reg.ServiceName]; !ok {
			p.services[reg.ServiceName] = make([]Registration, 0)
		}
		p.services[reg.ServiceName] = append(p.services[reg.ServiceName], reg)
	}

	for _, reg := range patch.Removed {
		log.Println("Removing service: ", reg.ServiceName, reg.ServiceID)
		if _, ok := p.services[reg.ServiceName]; !ok {
			continue
		}
		for i, r := range p.services[reg.ServiceName] {
			log.Println("Compare service ID: ", r.ServiceID, reg.ServiceID)
			if r.ServiceID == reg.ServiceID {
				p.services[reg.ServiceName] = append(p.services[reg.ServiceName][:i], p.services[reg.ServiceName][i+1:]...)
				break
			}
		}
	}
}

func (p *providers) get(name ServiceName) ([]Registration, error) {

	regs, ok := p.services[name]
	if !ok {
		return nil, fmt.Errorf("service %v not found", name)
	}

	return regs, nil
}

func GetProviders(name ServiceName) ([]Registration, error) {
	Prov.mutex.RLock()
	defer Prov.mutex.RUnlock()

	return Prov.get(name)
}

var Prov = providers{
	services: make(map[ServiceName][]Registration),
	mutex:    new(sync.RWMutex),
}
