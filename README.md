# ms3 — Mini Simple Storage Service

A lightweight, S3-compatible object storage server written in Go. Think MinIO, minus the operational weight — single
binary, single container, pluggable storage backends, and a CLI client for everyday use.

## Features

- S3-compatible REST API (buckets, objects, listing)
- CLI client (`ms3`) for managing buckets and objects from the terminal
- Pluggable storage backends — local filesystem or WebDAV
- Store any file/MIME type
- Presigned URLs with automatic expiration for secure, temporary downloads
- Dockerized — runs as a single container with `docker compose`

## Architecture

```mermaid
graph TB
    subgraph External["External Clients"]
        CLI["ms3c CLI"]
        SDK["aws-cli / S3 SDKs"]
        Postman["Postman / Apidog<br/>(manual testing)"]
    end

    subgraph DockerNet["Docker network: ms3-net"]
        subgraph APILayer["api-service (public entrypoint)"]
            API["REST :8080<br/>SigV4 verification<br/>presigned URL handling<br/>request orchestration"]
        end

        subgraph AuthLayer["auth-service (internal only)"]
            AuthAPI["REST :8082<br/>/v1/users, /v1/auth/*<br/>/internal/credentials/*"]
            AuthDB[("bbolt<br/>users, credentials")]
            AuthAPI --- AuthDB
        end

        subgraph MetaLayer["metadata-service (internal only)"]
            MetaAPI["REST :8081<br/>/v1/buckets/*"]
            MetaDB[("bbolt<br/>buckets, objects,<br/>owner index")]
            MetaAPI --- MetaDB
        end

        subgraph DataLayer["data-service (internal only)"]
            DataAPI["gRPC :9090<br/>PutObject/GetObject<br/>DeleteObject/StatObject"]
            DataKV[("bbolt/bbolt-KV<br/>storage_ref index")]
            Backend["Backend interface"]
            LocalFS[("Local filesystem<br/>or WebDAV")]
            DataAPI --- DataKV
            DataAPI --> Backend
            Backend --> LocalFS
        end
    end

    CLI -->|"HTTP, SigV4 signed"| API
    SDK -->|"HTTP, SigV4 signed"| API
    Postman -->|"HTTP"| API

    API -->|"REST: credential lookup<br/>GET /internal/credentials/{access_key}"| AuthAPI
    API -->|"REST: bucket/object metadata<br/>CRUD"| MetaAPI
    API -->|"gRPC: streamed bytes<br/>Put/Get/Delete/Stat"| DataAPI

    style API fill:#2b6cb0,color:#fff
    style AuthAPI fill:#805ad5,color:#fff
    style MetaAPI fill:#2f855a,color:#fff
    style DataAPI fill:#c05621,color:#fff
    style AuthDB fill:#f7fafc
    style MetaDB fill:#f7fafc
    style DataKV fill:#f7fafc

```

## License

[MIT](./LICENCE)
