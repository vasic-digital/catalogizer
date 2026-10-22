# Video 1.1: Course Overview - Script

**Duration**: 10 minutes
**Module**: 1 - Introduction and Architecture

---

## Scene 1: Introduction (0:00 - 1:30)

**[Visual: Title card with course name]**

**Narrator**: Welcome to "Mastering Catalogizer" - a comprehensive course on building a multi-platform media collection manager. I'm your instructor, and over the next 8-10 hours, we'll dive deep into every aspect of this production-ready application.

**[Visual: Show the running application]**

**Narrator**: Catalogizer is not just a demo project. It's a real, production-grade application used to manage media collections across multiple storage protocols - SMB, FTP, NFS, WebDAV, and local filesystems.

---

## Scene 2: What You'll Learn (1:30 - 3:30)

**[Visual: Animated list of learning objectives]**

**Narrator**: By the end of this course, you will be able to:

1. **Design scalable Go backends** with clean architecture patterns
2. **Build reactive frontends** with React 18 and TypeScript
3. **Implement protocol abstractions** that support multiple storage systems
4. **Create cross-platform applications** with Tauri for desktop
5. **Develop mobile applications** with Kotlin and Jetpack Compose
6. **Write comprehensive tests** with 95%+ coverage
7. **Deploy with containers** using Podman and docker-compose

**[Visual: Show architecture diagram]**

**Narrator**: We'll cover everything from the initial project structure to production deployment, including security best practices, performance optimization, and real-time features.

---

## Scene 3: Repository Structure (3:30 - 6:00)

**[Visual: Terminal showing directory tree]**

**Narrator**: Let's start by exploring the repository structure. Catalogizer is organized into several main components:

**[Visual: Highlight each directory]**

**Narrator**: 
- `catalog-api` - The Go backend with handlers, services, and repositories
- `catalog-web` - React frontend with TypeScript
- `catalogizer-desktop` - Tauri-based desktop application
- `catalogizer-android` and `catalogizer-androidtv` - Android mobile and TV apps
- `installer-wizard` - Guided installation application

**[Visual: Show submodule list]**

**Narrator**: The project uses 29 independent git submodules for shared libraries, allowing each component to be developed and versioned independently.

---

## Scene 4: Development Environment (6:00 - 9:00)

**[Visual: Terminal with commands]**

**Narrator**: Setting up your development environment is straightforward. First, clone the repository, then run the safe submodule initialiser (never a blanket `--recursive`, because a recursive update through the constitution folder cycles):

```bash
git clone https://github.com/anomalyco/catalogizer.git
cd catalogizer
./scripts/safe_submodule_init.sh   # never --recursive through constitution/
./scripts/install.sh --mode=development
```

**[Visual: Show Podman containers starting]**

**Narrator**: The project uses Podman for containerization. All builds and services run in containers with strict resource limits to ensure system stability.

**[Visual: Show running services]**

**Narrator**: To start the development environment:

```bash
# Terminal 1: Backend
cd catalog-api && go run main.go

# Terminal 2: Frontend
cd catalog-web && npm run dev
```

**[Visual: Browser showing localhost:3000]**

**Narrator**: Access the web UI at `http://localhost:3000` and the API at `http://localhost:8080`.

---

## Scene 5: Key Technologies (9:00 - 10:00)

**[Visual: Technology stack diagram]**

**Narrator**: The technology stack includes:

- **Backend**: Go 1.24, Gin web framework, SQLite/PostgreSQL, Redis
- **Frontend**: React 18, TypeScript, Vite, Tailwind CSS
- **Desktop**: Tauri 2.0 with Rust backend
- **Mobile**: Kotlin, Jetpack Compose, Room, Retrofit
- **Containerization**: Podman, podman-compose
- **Testing**: go test, Vitest, Playwright

**[Visual: Course title card]**

**Narrator**: In the next video, we'll dive into the system architecture and understand how all these components work together. Let's get started!

---

## Key Code Examples

### Cloning and Setup
```bash
git clone https://github.com/anomalyco/catalogizer.git
cd catalogizer
./scripts/safe_submodule_init.sh   # never --recursive through constitution/
./scripts/install.sh --mode=development
```

### Running Backend
```bash
cd catalog-api
go run main.go
# Server writes port to .service-port file
```

### Running Frontend
```bash
cd catalog-web
npm install
npm run dev  # Reads ../catalog-api/.service-port for API proxy
```

### Resource Limits
```bash
# Go tests with limited resources
GOMAXPROCS=3 go test ./... -p 2 -parallel 2
```

---

## Quiz Questions

1. What is the primary purpose of the `.service-port` file?
2. How many git submodules does the project use?
3. Why does the project use Podman instead of Docker?
4. What is the resource limit constraint for container operations?
