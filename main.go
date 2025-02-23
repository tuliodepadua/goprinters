package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Estrutura para resposta da API
type Device struct {
	IP   string `json:"ip"`
	Name string `json:"name"`
}

// Verifica se um IP responde ao ping
func isOnline(ip string) bool {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "100", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}
	err := cmd.Run()
	return err == nil
}

// Verifica se portas típicas de impressoras estão abertas
func scanPorts(ip string, ports []int) []int {
	var openPorts []int
	for _, port := range ports {
		address := fmt.Sprintf("%s:%d", ip, port)
		conn, err := net.DialTimeout("tcp", address, 1*time.Second)
		if err == nil {
			log.Printf("Porta %d aberta em %s\n", port, ip)
			openPorts = append(openPorts, port)
			conn.Close()
		}
	}
	return openPorts
}

// Tenta acessar a interface web da impressora para verificar se é Epson
func checkEpson(ip string) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:80", ip), 1*time.Second)
	if err == nil {
		defer conn.Close()
		buffer := make([]byte, 1024)
		conn.Read(buffer)
		return string(buffer) != "" && string(buffer) != "Desconhecido"
	}
	return false
}

// Verifica se um IP é uma Epson L3250
func identifyPrinter(ip string) bool {
	if !isOnline(ip) {
		return false
	}

	openPorts := scanPorts(ip, []int{80, 443, 515, 631, 9100})
	return checkEpson(ip) || contains(openPorts, 9100)
}

// Verifica se um slice contém um valor
func contains(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// Busca impressoras Epson na rede
func findEpsonPrinters(ips []string) []Device {
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var foundPrinters []Device

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			if identifyPrinter(ip) {
				mutex.Lock()
				foundPrinters = append(foundPrinters, Device{IP: ip, Name: "Epson L3250"})
				mutex.Unlock()
			}
		}(ip)
	}
	wg.Wait()
	return foundPrinters
}

// Endpoint para buscar impressoras Epson
func getPrinters(c *gin.Context) {
	log.Println("Buscando impressoras Epson na rede...")
	ips := []string{}
	for i := 1; i < 255; i++ {
		ips = append(ips, fmt.Sprintf("192.168.1.%d", i))
	}

	printers := findEpsonPrinters(ips)
	c.JSON(http.StatusOK, printers)
}

func main() {
	r := gin.Default()
	log.Println("Servidor iniciado...")

	// Criando endpoint
	r.GET("/api/printers", getPrinters)

	// Rodando o servidor na porta 8080
	fmt.Println("Servidor rodando em http://localhost:8080")
	r.Run("0.0.0.0:8080")
}
