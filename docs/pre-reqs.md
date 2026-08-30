## General Idea Gathering

1. [ ] Local registry
	1. Local Docker registry "Distribution": https://hub.docker.com/_/registry, running it as a container with volumes for persisting data
2. [ ] Containerize using Docker Compose
	1. [ ] Service for SQLite
	2. [ ] Service for Application
	3. [ ] Define a shared network between these services
	4. [ ] Define a volume for the SQLite
3. Application Logic idea:
	1. [Simplified MinIO](#project-design)
4. Deploy on Kubernetes

---

## Project Design

- Separate each service in a directory and each one has its own docker config
	- Auth service
 		- TODO
	- Api service
 		- The idea for the API Service is to be as simple as possible and its main goal is to distribute requests for each corresponding services which means that clients should be responsible for configuring the HTTP requests for API service to work.
	- Data service
	- Metadata service

- Core operations
	- pre-signed URLs
	- Upload objects
	- Get / Download objects
 	- List buckets
 	- User Authentication

---
### References:
1. https://ivov.dev/notes/s3-object-storage
