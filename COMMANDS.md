# List of Commands for testing purposes

**POST new hospital patient**
`curl -k -X POST https://localhost:8081/patient -d '8082'`

**Create RSA key**
`openssl genrsa -out server.key 2048`

**Create Certificate from RSA key**
`openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650 -addext "subjectAltName = DNS:localhost"`
