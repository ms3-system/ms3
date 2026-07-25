1. [ ] Local registry
	1. Local Docker registry "Distribution": https://hub.docker.com/_/registry, running it as a container with volumes for persisting data
2. [ ] Containerize using Docker Compose
	1. [ ] Service for SQLite
	2. [ ] Service for Application
	3. [ ] Define a shared network between these services
	4. [ ] Define a volume for the SQLite
3. Application Logic idea:
	1. Simplified MinIO
4. [ ] #Old - Do we need env in our project to use `Vault`?
---
1. **Path-Style:** `http://localhost:9090/bucket-name/object-key`
	1. No ==DNS== required
2. Authenticating Users
3. 
