# TOWD
Tiny Office With Discord

## Development
### Prerequisites
- [Go](https://go.dev/)
- [pnpm](https://pnpm.io/)
- [requiredfield](https://github.com/abhinav/requiredfield/)
- Discord Bot Token, Groq API Key

### Development
> To bypass the browser's restriction that requires enabling `Secure` when using `SameSite=None` for cross-site cookie authentication, this project uses Caddy to proxy requests with `tls internal`. This avoids the manual hassle of creating, signing, and installing certificates.
>
> Additionally, Caddy rewrites/overwrites certain headers to enforce strict cross-site cookie policies. For production builds, the frontend is generated as a static site, and the backend serves the frontend assets directly, eliminating the need for cross-site cookies.

#### There are 3 places to configure the ports:
- `Caddyfile`: proxies frontend and backend dev servers with a certificate (default `3001` -> `3000` for frontend; `8081` -> `8080` for backend)
- `.env`:
    - `PORT`: frontend dev server listening port (default `3000`)
    - `SERVER_PORT`: backend server listening port (default `8080`)
    - `VITE_SERVER_HOSTNAME`: where the frontend should reach the backend, this must be configured to point to the hostname and port where Caddy proxies the backend (default `https://localhost:8081`).
- `nuxt.config.ts`: `vite.server.hmr.clientPort` must be the same as the port listening port of `vite`, not after proxied through Caddy (default `3000`)

#### Commonly used scripts:
- `go run .`: backend dev server listen on `SERVER_PORT` (`8080`) port
- `pnpm dev`: frontend dev server listen on `PORT` (`3000`) port
- `caddy run --config ./Caddyfile`: proxy those 2 servers

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