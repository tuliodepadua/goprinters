package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grandcat/zeroconf"
)

// Estrutura para resposta da API
type Printer struct {
	Name string   `json:"name"`
	IP   []string `json:"ip"`
}

// Função que busca impressoras na rede via mDNS
func findPrinters() ([]Printer, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var printers []Printer

	go func() {
		for entry := range entries {
			var ips []string
			for _, ip := range entry.AddrIPv4 {
				ips = append(ips, ip.String())
			}

			printers = append(printers, Printer{
				Name: entry.ServiceInstanceName(),
				IP:   ips,
			})
		}
	}()

	err = resolver.Browse(ctx, "_ipp._tcp", "local.", entries)
	if err != nil {
		return nil, err
	}

	<-ctx.Done()
	return printers, nil
}

// Endpoint para buscar impressoras
func getPrinters(c *gin.Context) {
	log.Println("Buscando impressoras...")
	printers, err := findPrinters()
	if err != nil {
		log.Println("Erro ao buscar impressoras:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
