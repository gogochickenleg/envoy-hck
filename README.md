# Envoy with mTLS and Go gRPC App

This project demonstrates how to use Envoy as a proxy with mutual TLS (mTLS) and gRPC health checking for a Go application.

## Prerequisites

- [Go](https://golang.org/doc/install) installed.
- [Protocol Buffers v3](https://grpc.io/docs/protoc-installation/) installed.
- Go plugins for protocol buffers:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
  ```
- [Docker](https://docs.docker.com/get-docker/) to run Envoy.
- `openssl` command-line tool.

## Setup

1.  **Create Project Structure:**

    ```bash
    mkdir -p your_project_name/protos
    cd your_project_name
    go mod init your_project_name
    ```

2.  **Save the Project Files:**

    - Save the Envoy configuration as `envoy.yaml`.
    - Save the Go application code as `main.go`.
    - Save the protobuf definition as `protos/time.proto`.

3.  **Generate Certificates:**
    Follow the instructions in the "Certificate Generation Commands" document to create the `certs` directory and all necessary keys and certificates.

4.  **Generate Go code from Protobuf:**

    ```bash
    protoc --go_out=. --go_opt=paths=source_relative \
        --go-grpc_out=. --go-grpc_opt=paths=source_relative \
        protos/time.proto
    ```

5.  **Install Go Dependencies:**
    ```bash
    go mod tidy
    ```

## Running the Application

1.  **Start the Go Application:**
    In one terminal, run the Go server. It will automatically load the certificates from the `certs` directory.

    ```bash
    go run main.go
    ```

2.  **Start Envoy:**
    In a second terminal, run Envoy using Docker. Note the new `-v` flag to mount the `certs` directory into the container so Envoy can access them.
    ```bash
    docker run --rm -it -p 8080:8080 -p 9901:9901 --network="host" \
        -v $(pwd)/envoy.yaml:/etc/envoy/envoy.yaml \
        -v $(pwd)/certs:/etc/envoy/certs \
        envoyproxy/envoy:v1.22.0
    ```

## Testing

To test the mTLS connection, you need a gRPC client that can also present the correct certificates. `grpcurl` is perfect for this.

1.  **Install `grpcurl`:**

    ```bash
    go install [github.com/fullstorydev/grpcurl/cmd/grpcurl@latest](https://github.com/fullstorydev/grpcurl/cmd/grpcurl@latest)
    ```

2.  **Call the Service with TLS:**
    This command tells `grpcurl` to act as a client, presenting the `client.crt` and trusting the `ca.crt`.
    ```bash
    grpcurl \
        -cacert certs/ca.crt \
        -cert certs/client.crt \
        -key certs/client.key \
        -d '{}' \
        localhost:8080 time.TimeService/StreamTime
    ```
    You should see the time streaming successfully. If you try to run it without the certificates, the connection will be rejected by Envoy.

### Certificate Generation for mTLS

These `openssl` commands will create a self-signed Certificate Authority (CA) and use it to issue certificates for your Go application (the "server") and Envoy (the "client").

1.  **Create a directory for the certificates:**

    ```bash
    mkdir certs
    cd certs
    ```

2.  **Create the Certificate Authority (CA):**

    - Generate the CA's private key:
      ```bash
      openssl genrsa -out ca.key 4096
      ```
    - Generate the CA's root certificate. You'll be prompted for information; you can accept the defaults.
      ```bash
      openssl req -x509 -new -nodes -key ca.key -sha256 -days 1024 -out ca.crt -subj "/CN=my-ca"
      ```

3.  **Create the Server Certificate (for the Go App):**

    - Generate the server's private key:
      ```bash
      openssl genrsa -out server.key 4096
      ```
    - Create a Certificate Signing Request (CSR) for the server.
      ```bash
      openssl req -new -key server.key -out server.csr -subj "/CN=localhost"
      ```
    - Sign the server certificate with your CA:
      ```bash
      openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 500 -sha256
      ```

4.  **Create the Client Certificate (for Envoy):**

    - Generate the client's private key:
      ```bash
      openssl genrsa -out client.key 4096
      ```
    - Create a CSR for the client.
      ```bash
      openssl req -new -key client.key -out client.csr -subj "/CN=envoy"
      ```
    - Sign the client certificate with your CA:
      ```bash
      openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 500 -sha256
      ```

5.  **Return to the project root directory:**
    ```bash
    cd ..
    ```

After running these commands, your `certs` directory should contain `ca.crt`, `server.crt`, `server.key`, `client.crt`, and `client.key`, among other files.


## Quick Start Guide

1. **Start Envoy Proxy:**
   ```bash
   docker run --rm -it -p 8080:8080 -p 9901:9901 \
       -v $(pwd)/envoy.yml:/etc/envoy/envoy.yaml \
       envoyproxy/envoy:v1.22.0
   ```

   **Note:** After running, it takes some time for Envoy to start up. You can verify it's running by checking the admin interface at http://localhost:9901/clusters - the output should contain `health_flags` when ready.

2. **Start the Go gRPC Server:**
   ```bash
   go run main.go
   ```

3. **Toggle Health Status (Optional):**
   ```bash
   curl http://localhost:8081/toggle-health
   ```

4. **Test the gRPC Service:**
   ```bash
   grpcurl -d '{}' localhost:8080 time.TimeService/StreamTime
   ```