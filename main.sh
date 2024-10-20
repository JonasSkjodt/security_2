#!/usr/bin/env bash

go run hospital/hospital.go & :
go run patient/patient.go -port=8081 & :
go run patient/patient.go -port=8082 & : 
go run patient/patient.go -port=8083