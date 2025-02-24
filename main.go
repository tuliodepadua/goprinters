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
	"github.com/gosnmp/gosnmp"
)

// Estrutura para resposta da API
type Device struct {
	IP   string `json:"ip"`
	Name string `json:"name"`
}

func getPrinterInfo(ip string) {
	// Configuração SNMP (Versão 2c e comunidade "public")
	snmp := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: "public",
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
		Retries:   3,
	}

	// Inicia a conexão SNMP
	err := snmp.Connect()
	if err != nil {
		log.Printf("Erro ao conectar a %s: %v\n", ip, err)
		return
	}
	defer snmp.Conn.Close()

	// OIDs comuns de impressoras
	oids := map[string]string{
		"Nome do Dispositivo": "1.3.6.1.2.1.1.5.0",           // sysName
		"Modelo":              "1.3.6.1.2.1.25.3.2.1.3.1",    // hrDeviceDescr
		"Número de Páginas":   "1.3.6.1.2.1.43.10.2.1.4.1.1", // OID genérico de contagem de páginas
		"Nível de Toner":      "1.3.6.1.2.1.43.11.1.1.9.1.1", // OID genérico para toner
	}

	// Faz requisição SNMP para cada OID
	for desc, oid := range oids {
		result, err := snmp.Get([]string{oid})
		if err != nil {
			log.Printf("[%s] Erro ao buscar %s: %v\n", ip, desc, err)
			continue
		}

		// Exibe os resultados
		for _, variable := range result.Variables {
			fmt.Printf("[%s] %s: %v\n", ip, desc, variable.Value)
		}
	}
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

func identifyPrinter(ip string) bool {
	if !isOnline(ip) {
		return false
	}

	openPorts := scanPorts(ip, []int{80, 443, 515, 631, 9100})
	return checkEpson(ip) || contains(openPorts, 9100)
}

func contains(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func findPrinters(ips []string) []Device {
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var foundPrinters []Device

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			if identifyPrinter(ip) {
				mutex.Lock()
				getPrinterInfo(ip)
				foundPrinters = append(foundPrinters, Device{IP: ip, Name: "Possível impressora"})
				mutex.Unlock()
			}
		}(ip)
	}
	wg.Wait()
	return foundPrinters
}

func getPrinters(c *gin.Context) {
	log.Println("Buscando impressoras na rede...")
	ips := []string{}
	for i := 1; i < 255; i++ {
		ips = append(ips, fmt.Sprintf("192.168.1.%d", i))
	}

	printers := findPrinters(ips)
	c.JSON(http.StatusOK, printers)
}

func main() {
	r := gin.Default()
	log.Println("Servidor iniciado...")

	r.GET("/api/printers", getPrinters)

	fmt.Println("Servidor rodando em http://localhost:8080")
	r.Run("0.0.0.0:8080")
}
