package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-distributed/registry/heartbeat"
	"go-distributed/utils"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

func RegisterService(r Registration) error {
	serviceUpdatedURL, err := url.Parse(r.ServiceUpdateURL)
	if err != nil {
		return err
	}

	log.Println("Service update URL Path: ", serviceUpdatedURL.Path)
	http.Handle(serviceUpdatedURL.Path, &serviceUpdateHandler{})

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	err = enc.Encode(r)

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
			break
		}
		log.Println("Failed to register service. Retry after 3 seconds...")
		time.Sleep(3 * time.Second)
	}

	log.Println("Service registered successfully")

	// Start sending heartbeats
	interval := 3 * time.Second

	registryHeartbeatURL := "http://localhost:3000/heartbeat/basic"

	hb := heartbeat.BasicHeartbeat{
		URL: registryHeartbeatURL,
	}

	go func() {
		for {
			hb.SendHeartbeat()
			time.Sleep(interval)
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
	prov.Update(p)
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
	services map[ServiceName][]string
	mutex    *sync.RWMutex
}

func (p *providers) Update(patch patch) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, patchEntry := range patch.Added {
		if _, ok := p.services[patchEntry.Name]; !ok {
			p.services[patchEntry.Name] = make([]string, 0)
		}

		p.services[patchEntry.Name] = append(p.services[patchEntry.Name], patchEntry.URL)
	}

	for _, patchEntry := range patch.Removed {
		if urls, ok := p.services[patchEntry.Name]; ok {
			for i := range urls {
				if urls[i] == patchEntry.URL {
					p.services[patchEntry.Name] = append(urls[:i], urls[i+1:]...)
				}
			}
		}
	}
}

func (p *providers) get(name ServiceName) ([]string, error) {

	urls, ok := p.services[name]
	if !ok {
		return nil, fmt.Errorf("service %v not found", name)
	}

	return urls, nil
}

func GetProviders(name ServiceName) ([]string, error) {
	prov.mutex.RLock()
	defer prov.mutex.RUnlock()

	return prov.get(name)
}

var prov = providers{
	services: make(map[ServiceName][]string),
	mutex:    new(sync.RWMutex),
}
