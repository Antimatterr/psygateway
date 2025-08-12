package discovery

import (
	"fmt"

	"github.com/Antimatterr/psygateway/internal/logger"
	"github.com/hashicorp/consul/api"
)

type ServiceDiscovery struct {
	client *api.Client
}

func NewServiceDiscovery(consulAddress string) (*ServiceDiscovery, error) {
	if consulAddress == "" {
		logger.Warn("CONSUL_ADDRESS environment variable is not set")
		consulAddress = "localhost:8500" // default value
	}

	config := api.DefaultConfig()
	config.Address = consulAddress

	client, err := api.NewClient(config)
	if err != nil {
		logger.Error("Failed to create Consul client", err)
		return nil, err
	}

	return &ServiceDiscovery{client: client}, nil
}

func (sd *ServiceDiscovery) RegisterService(serviceName, address string, port int, healthCheckPath string) error {
	registeration := &api.AgentServiceRegistration{
		ID:      fmt.Sprintf("%s-%s", serviceName, address),
		Name:    serviceName,
		Address: address,
		Port:    port,
		Check: &api.AgentServiceCheck{
			HTTP:                           fmt.Sprintf("http://%s:%d%s", address, port, healthCheckPath),
			Interval:                       "10s",
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "1m",
		},
	}

	return sd.client.Agent().ServiceRegister(registeration)
}

func (sd *ServiceDiscovery) DeregisterService(serviceID string) error {
	return sd.client.Agent().ServiceDeregister(serviceID)
}

func (sd *ServiceDiscovery) GetHealthyService(serviceName string) (string, error) {
	services, _, err := sd.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		logger.Error("Failed to get healthy service", err, "service", serviceName)
		return "", err
	}

	if len(services) == 0 {
		return "", fmt.Errorf("no healthy services found for %s", serviceName)
	}

	//need load balancing here as there can be multiple healthy services instances
	//for simplicity, returning the first healthy service instance

	// Return full HTTP URL
	serviceURL := fmt.Sprintf("http://%s:%d", services[0].Service.Address, services[0].Service.Port)
	logger.Info("Found healthy service", "service", serviceName, "url", serviceURL)

	return serviceURL, nil
}
