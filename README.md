# TOWD
Tiny Office With Discord

## Development
### Prerequisites
- [Go](https://go.dev/)
- [Bun](https://bun.sh/)
- [requiredfield](https://github.com/abhinav/requiredfield/)
- Discord Bot Token, Groq API Key

### Development server
- Backend: `go run .`
- Frontend: `bun dev`

## Build, deployment and update
### Prerequisites
- Install [Docker Engine](https://docs.docker.com/engine/install/) or [Podman](https://podman.io/)
- `cp docker-compose.example.yml docker-compose.yml`
- Modify environment variables in `docker-compose.yml`

### Deployment
- `docker compose up -d`

### Update
- `docker compose down`
- `git pull --rebase`
- `docker compose up -d`