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
var aggregateShare int
var totalPatients int
var sharesReceived []int
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

// creates server for patient
func patientServer() {
	log.Printf("Creating patient server for: %d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/patients", Patients)
	mux.HandleFunc("/shares", Shares)

	err := http.ListenAndServeTLS(FormatPort(port), "server.crt", "server.key", mux)
	if err != nil {
		log.Fatal(err)
	}
}

// Patients handles the POST request from the hospital
func Patients(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		log.Println(port, ": Patient received POST /patients")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("%d: Error reading request body: %v", port, err), http.StatusInternalServerError)
			return
		}

		var patients Patient
		if err := json.Unmarshal(body, &patients); err != nil {
			http.Error(w, fmt.Sprintf("%d: Error unmarshalling patients: %v", port, err), http.StatusInternalServerError)
			return
		}
		
		shares := GenerateShares(maxRanVal, data, totalPatients) 

		log.Println(port, ": Patient received list of patients:", patients.PortsList)
		// Send shares to patients
		for index, shareValue := range shares {
			if index == totalPatients-1 {
				break
			}
			go func(index, shareValue int) {
				share := Share{
					Share: shareValue,
				}
				shareBytes, err := json.Marshal(share)
				if err != nil {
					http.Error(w, fmt.Sprintf("%d: Error when marshalling share during /patients: %v", port, err), http.StatusInternalServerError)
					return
				}
				url := fmt.Sprintf("https://localhost:%d/shares", patients.PortsList[index])
				response, err := client.Post(url, "application/json", bytes.NewReader(shareBytes))
				if err != nil {
					log.Println(port, ": Error when sending share to", patients.PortsList[index], ":", err)
					return
				}
				log.Println(port, ": Sent share to", patients.PortsList[index], ". Received response code:", response.StatusCode)
			}(index, shareValue)
		}
		// Append the last share to the receivedShares list
		sharesReceived = append(sharesReceived, shares[len(shares)-1])

		handleReceivedShare(w)
	}
}

func Shares(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		log.Println(port, ": Patient received POST /shares")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("%d: Error when reading request body during /shares: %v", port, err), http.StatusInternalServerError)
			return
		}
		receivedShare := &Share{}
		err = json.Unmarshal(body, receivedShare)
		if err != nil {
			http.Error(w, fmt.Sprintf("%d: Error when unmarshalling share: %v", port, err), http.StatusInternalServerError)
			return
		}
		// Append the received share to the receivedShares list
		sharesReceived = append(sharesReceived, receivedShare.Share)

		handleReceivedShare(w)
	}
}

func sendAggregateShare() {
	log.Println(port, ": Computing aggregate share")

	for _, share := range sharesReceived {
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


// GenerateShares with n-out-of-n additive secret share
func GenerateShares(p int, data int, amount int) []int {
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

func FormatPort(port int) string {
	return fmt.Sprintf(":%d", port)
}

// Helper function which checks if all shares have been received and sends the aggregate share to the hospital
func handleReceivedShare(w http.ResponseWriter) {
	// Check if all shares have been received
	if len(sharesReceived) == totalPatients {
		sendAggregateShare()
	}

	// Respond with status OK
	w.WriteHeader(http.StatusOK)
}
