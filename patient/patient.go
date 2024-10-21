package main

import (
	"fmt"
	"flag"
	"bytes"
	"encoding/json"
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"
	"net/http"
	"log"
	"math/rand"
	"time"
)

type Patient struct {
	Port int
	PortsList []int
}

type Share struct {
	Share int
}

var client *http.Client
var data int
var hospitalPort int
var port int
var maxRanVal int
var totalPatients int
var receivedShares []int
var maxCompVal int



func main() {

	flag.IntVar(&port, "port", 8081, "port for patient")
	flag.IntVar(&hospitalPort, "h", 8080, "port of the hospital")
	flag.IntVar(&totalPatients, "t", 3, "the total amount of patients")
	flag.IntVar(&maxCompVal, "maxCompVal", 500, "the max value that the final computation can have")

	flag.Parse()

	maxRanVal = maxCompVal / 3

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	data = r.Intn(maxRanVal)

	log.Println(port, ": New patient with data =", data)

	// Load in the certification from file server.crt
	cert, err := os.ReadFile("server.crt")
	if err != nil {
		log.Fatal(err)
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cert)

	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}

	go patientServer() // Run the server

	// Register patient with hospital
	log.Println(port, ": Patient registering with hospital")

	url := fmt.Sprintf("https://localhost:%d/patient", hospitalPort)

	ownPort := Patient{
		Port: port,
	}

	b, err := json.Marshal(ownPort)
	if err != nil {
		log.Fatal(port, ": Error when marshalling patient:", err)
	}

	response, err := client.Post(url, "string", bytes.NewReader(b))
	if err != nil {
		log.Fatal(port, ": Error when regisering with hospital:", err)
	}
	log.Println(port, ": Registered with hospital, received response code", response.Status)

	select {} // Keep the server running
}

// List of ports of other patients provided by hospital
func Patients(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		log.Println(port, ": Patient received POST /patients")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatal(port, ": Error when reading request body during /patients:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		patients := &Patient{}
		err = json.Unmarshal(body, patients)
		if err != nil {
			log.Fatal(port, ": Error when unmarshalling patients:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		shares := CreateShares(maxRanVal, data, totalPatients) 

		log.Println(port, ": Sending shares to other patients")
		for i, share := range shares { // Send a share to each other patient
			if i == totalPatients-1 {
				break
			}
			ownShare := Share{
				Share: share,
			}
			b, err := json.Marshal(ownShare)
			if err != nil {
				log.Fatal(port, ": Error when marshalling share during /patients:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			url := fmt.Sprintf("https://localhost:%d/shares", patients.PortsList[i])
			response, err := client.Post(url, "string", bytes.NewReader(b))
			if err != nil {
				log.Fatal(port, ": Error when sending share to", patients.PortsList[i], ":", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			log.Println(port, ": Sent share to, ", patients.PortsList[i], ". Received response code:", response.StatusCode)
		}

		receivedShares = append(receivedShares, shares[len(shares)-1]) // Add own share to list of receivedShares

		if len(receivedShares) == totalPatients {
			sendAggregateShare()
		}

		w.WriteHeader(http.StatusOK)

	}
}

func Shares(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		log.Println(port, ": Patient received POST /shares")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatal(port, ": Error when reading request body during /shares:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		foreignShare := &Share{}
		err = json.Unmarshal(body, foreignShare)
		if err != nil {
			log.Fatal(port, ": Error when unmarshalling share:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		receivedShares = append(receivedShares, foreignShare.Share) // Add received share to list of received shares

		if len(receivedShares) == totalPatients {
			sendAggregateShare()
		}

		w.WriteHeader(http.StatusOK)
	}
}

func sendAggregateShare() {
	log.Println(port, ": Computing aggregate share")
	// If all shares have been received,
	// calculate and aggregate share and send it to the hospital

	var aggregateShare int

	for _, share := range receivedShares {
		aggregateShare = aggregateShare + share
	}

	log.Println(port, ": aggregate share is", aggregateShare)

	aggregate := Share{
		Share: aggregateShare,
	}

	b, err := json.Marshal(aggregate)
	if err != nil {
		log.Fatal(port, ": Error when marshalling aggregate share during /shares:", err)
		return
	}

	log.Println(port, ": Sending aggregate share", aggregateShare, "to hospital")
	url := fmt.Sprintf("https://localhost:%d/shares", hospitalPort)
	response, err := client.Post(url, "string", bytes.NewReader(b))
	if err != nil {
		log.Fatal(port, ": Error when sending aggregate share to hospital:", err)
		return
	}
	log.Println(port, ": Sent aggregate share to hospital, received response code", response.StatusCode)
}

func patientServer() {
	log.Println(port, ": Creating patient server")

	mux := http.NewServeMux()
	mux.HandleFunc("/patients", Patients)
	mux.HandleFunc("/shares", Shares)

	err := http.ListenAndServeTLS(StringifyPort(port), "server.crt", "server.key", mux)
	if err != nil {
		log.Fatal(err)
	}

}

func StringifyPort(port int) string {
	return fmt.Sprintf(":%d", port)
}

func CreateShares(p int, data int, amount int) []int {
	var shares []int
	var totalShares int

	for i := 0; i < amount-1; i++ {
		share := rand.Intn(p-1) + 1
		shares = append(shares, share)
		totalShares += share
	}

	shares = append(shares, data-totalShares)

	return shares
}

