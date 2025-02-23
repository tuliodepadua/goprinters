package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grandcat/zeroconf"
)

// Estrutura para resposta da API
type Device struct {
	IP   string `json:"ip"`
	Name string `json:"name"`
}

// Busca dispositivos ativos via mDNS (Zeroconf)
func findMDNSDevices() ([]Device, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var devices []Device
	var mu sync.Mutex

	go func() {
		for entry := range entries {
			for _, ip := range entry.AddrIPv4 {
				mu.Lock()
				devices = append(devices, Device{
					Name: entry.ServiceInstanceName(),
					IP:   ip.String(),
				})
				mu.Unlock()
			}
		}
	}()

	err = resolver.Browse(ctx, "_services._dns-sd._udp", "local.", entries)
	if err != nil {
		return nil, err
	}

	<-ctx.Done()

	return devices, nil
}

// Escaneia IPs na rede e lista dispositivos ativos
func scanActiveDevices() []Device {
	subnet := "192.168.1" // Ajuste conforme sua rede
	var devices []Device
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 1; i <= 254; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := fmt.Sprintf("%s.%d", subnet, i)

			if ping(ip) {
				hostname := getHostname(ip)
				mu.Lock()
				devices = append(devices, Device{
					IP:   ip,
					Name: hostname,
				})
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	return devices
}

// Testa se um IP está ativo via ping
func ping(ip string) bool {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "100", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}

	err := cmd.Run()
	return err == nil
}

// Obtém o hostname do dispositivo pelo IP
func getHostname(ip string) string {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return "Desconhecido"
	}
	return names[0]
}

// Busca dispositivos na rede (via mDNS e ping)
func findDevices() ([]Device, error) {
	mdnsDevices, _ := findMDNSDevices()
	pingDevices := scanActiveDevices()

	// Unindo os resultados e removendo duplicatas
	deviceMap := make(map[string]Device)

	for _, d := range mdnsDevices {
		deviceMap[d.IP] = d
	}

	for _, d := range pingDevices {
		if _, exists := deviceMap[d.IP]; !exists {
			deviceMap[d.IP] = d
		}
	}

	var devices []Device
	for _, d := range deviceMap {
		devices = append(devices, d)
	}

	return devices, nil
}

// Endpoint para buscar dispositivos
func getDevices(c *gin.Context) {
	log.Println("Buscando dispositivos ativos na rede...")
	devices, err := findDevices()
	if err != nil {
		log.Println("Erro ao buscar dispositivos:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devices)
}

func main() {
	r := gin.Default()
	log.Println("Servidor iniciado...")

	// Criando endpoint
	r.GET("/api/devices", getDevices)

	// Rodando o servidor na porta 8080
	fmt.Println("Servidor rodando em http://localhost:8080")
	r.Run("0.0.0.0:8080")
}
