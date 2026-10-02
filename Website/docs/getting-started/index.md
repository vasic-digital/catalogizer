# Getting Started with Catalogizer

Welcome to Catalogizer! This guide will help you get up and running with Catalogizer on your system.

## System Requirements

### Minimum Requirements
- **Operating System**: Windows 10, macOS 10.15, or Ubuntu 20.04
- **RAM**: 4 GB RAM
- **Storage**: 10 GB free disk space
- **Network**: Internet connection for installation

### Recommended Requirements
- **Operating System**: Latest version of Windows, macOS, or Linux
- **RAM**: 8 GB RAM or more
- **Storage**: 50 GB free disk space
- **Network**: Gigabit network for media servers

### Supported Platforms
- **Desktop**: Windows, macOS, Linux
- **Mobile**: Android 8.0+, Android TV
- **Server**: Docker (Linux containers)

## Installation Options

### Option 1: Installation Wizard (Recommended for beginners)

The installation wizard provides a graphical interface for setting up Catalogizer with all necessary dependencies.

#### Step 1: Download the Installer
1. Visit the [Catalogizer Releases](https://github.com/catalogizer/catalogizer/releases)
2. Download the installer for your platform:
   - Windows: `Catalogizer-Installer-Windows.exe`
   - macOS: `Catalogizer-Installer-Mac.dmg`
   - Linux: `Catalogizer-Installer-Linux.run`

#### Step 2: Run the Installer
1. Double-click the downloaded installer
2. Follow the wizard prompts:
   - Accept the license agreement
   - Choose installation location
   - Select components to install
   - Configure initial settings
3. Wait for installation to complete
4. Launch Catalogizer from the desktop shortcut

#### Step 3: Initial Configuration
1. The Configuration Wizard will launch automatically
2. Follow these steps:
   - **Database Setup**: Choose SQLite (default) or configure external database
   - **Media Sources**: Add folders or network shares containing media
   - **User Account**: Create your admin account
   - **Network Settings**: Configure port and access options
3. Complete the wizard and Catalogizer will start

### Option 2: Docker Installation (Recommended for advanced users)

Docker provides an isolated environment that's easy to manage and deploy.

#### Prerequisites
- Docker and Docker Compose installed
- Command line / terminal access

#### Installation Steps
1. **Download Docker Compose file**:
   ```bash
   curl -O https://raw.githubusercontent.com/catalogizer/catalogizer/main/docker-compose.yml
   ```

2. **Create environment file**:
   ```bash
   cp docker-compose.yml.example docker-compose.yml
   # Edit docker-compose.yml with your settings
   ```

3. **Start Catalogizer**:
   > Uses rootless Podman Compose. Never use sudo. Release artifacts are built only inside rootless build containers (see the project constitution, §11.4.173).

   ```bash
   podman compose up -d
   ```

4. **Access Catalogizer**:
   - Web UI: http://localhost:8080
   - API: http://localhost:8080/api/v1

### Option 3: Manual Installation (For developers)

#### Backend Installation
1. **Install Go 1.25+**
   ```bash
   # macOS
   brew install go
   
   # Ubuntu
   sudo apt-get install golang-go
   
   # Windows
   # Download from https://golang.org/dl/
   ```

2. **Clone Repository**
   ```bash
   git clone https://github.com/catalogizer/catalogizer.git
   cd catalogizer/catalog-api
   ```

3. **Install Dependencies**
   ```bash
   go mod tidy
   ```

4. **Run Backend**
   ```bash
   go run main.go
   ```

#### Frontend Installation
1. **Install Node.js 18+**
   ```bash
   # macOS
   brew install node
   
   # Ubuntu
   sudo apt-get install nodejs npm
   
   # Windows
   # Download from https://nodejs.org/
   ```

2. **Navigate to Frontend Directory**
   ```bash
   cd catalog-web
   ```

3. **Install Dependencies**
   ```bash
   npm install
   ```

4. **Run Frontend**
   ```bash
   npm run dev
   ```

## First Time Setup

### 1. Access Catalogizer
After installation, access Catalogizer in your browser:
- Local installation: http://localhost:5173
- Docker installation: http://localhost:8080

### 2. Create Admin Account
1. You'll be prompted to create an admin account
2. Fill in:
   - Username
   - Email address
   - Password (use a strong password)
3. Click "Create Account"

### 3. Add Media Sources
1. Navigate to Settings → Storage
2. Click "Add Source"
3. Choose source type:
   - **Local Folder**: Browse to a folder on your computer
   - **SMB Share**: Enter server details (e.g., `smb://server/share`)
   - **FTP Server**: Enter FTP server details
   - **NFS Mount**: Enter NFS mount point
   - **WebDAV**: Enter WebDAV URL
4. Configure authentication if required
5. Test connection
6. Save the source

### 4. Initial Media Scan
1. Go to Dashboard
2. Click "Scan Now" or wait for automatic scan
3. Catalogizer will:
   - Scan all configured sources
   - Detect media files
   - Extract metadata
   - Generate thumbnails
4. This may take time for large libraries

## Next Steps

Congratulations! You now have Catalogizer installed and running. Here are some next steps:

- [Browse your media library](../user-guide/media-browsing.md)
- [Learn about collections](../user-guide/collections.md)
- [Set up automatic organization](../user-guide/automation.md)
- [Install mobile apps](../user-guide/mobile-setup.md)
- [Configure advanced settings](../user-guide/settings.md)

## Troubleshooting

### Installation Issues

#### Port Already in Use
If you see "Port 8080 is already in use":
```bash
# Find process using port 8080
lsof -i :8080

# Kill the process (replace PID with actual process ID)
kill -9 PID
```

#### Permission Denied
If you get permission errors:
```bash
# Linux/macOS
sudo chown -R $USER:$USER /path/to/catalogizer

# Windows (run as Administrator)
# Re-run installer as Administrator
```

#### Database Connection Failed
1. Check if database service is running
2. Verify database connection settings
3. Check firewall settings

### First Run Issues

#### No Media Found
1. Verify media sources are configured correctly
2. Check network connectivity for remote sources
3. Ensure authentication credentials are correct

#### Slow Performance
1. Check system resources (CPU, RAM, disk)
2. Optimize media source locations
3. Adjust scanning frequency in settings

## Need Help?

- [Documentation](../user-guide/)
- [Community Forum](https://forum.catalogizer.com)
- [Discord Server](https://discord.gg/catalogizer)
- [Report Issues](https://github.com/catalogizer/catalogizer/issues)

---

This getting started guide is part of the complete Catalogizer documentation. For more detailed information, explore the other sections in this documentation site.