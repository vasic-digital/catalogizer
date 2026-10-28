#!/usr/bin/env bash
#
# Catalogizer Full Distribution Script
# Builds images and distributes them to remote hosts
#
# This script:
# 1. Builds container images locally
# 2. Saves images to tar files
# 3. Transfers them to remote hosts
# 4. Loads images on remote hosts
# 5. Starts services on remote hosts
#
# Usage:
#   ./scripts/full-distribute.sh [--build] [--transfer] [--start] [--all]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/submodules/containers/.env"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Parse env file
parse_env() {
    if [[ -f "${ENV_FILE}" ]]; then
        source <(grep -v '^#' "${ENV_FILE}" | sed 's/^/export /')
    fi
}

# Get remote hosts from env
get_remote_hosts() {
    local hosts=""
    local i=1
    while true; do
        var_name="CONTAINERS_REMOTE_HOST_${i}_NAME"
        if [[ -n "${!var_name:-}" ]]; then
            name="${!var_name}"
            addr_var="CONTAINERS_REMOTE_HOST_${i}_ADDRESS"
            user_var="CONTAINERS_REMOTE_HOST_${i}_USER"
            addr="${!addr_var:-localhost}"
            user="${!user_var:-$USER}"
            hosts="${hosts} ${user}@${addr}"
            ((i++))
        else
            break
        fi
    done
    echo "${hosts}" | xargs
}

build_images() {
    echo -e "${BLUE}=== Building Container Images ===${NC}"
    
    # Build API image
    echo -e "${YELLOW}Building catalog-api...${NC}"
    cd "${PROJECT_ROOT}/catalog-api"
    podman build -t catalogizer-api:latest -f Dockerfile .
    
    # Build Web image
    echo -e "${YELLOW}Building catalog-web...${NC}"
    cd "${PROJECT_ROOT}/catalog-web"
    podman build -t catalogizer-web:latest -f Dockerfile .
    
    echo -e "${GREEN}Build complete!${NC}"
}

transfer_images() {
    echo -e "${BLUE}=== Transferring Images to Remote Hosts ===${NC}"
    
    hosts=$(get_remote_hosts)
    if [[ -z "${hosts}" ]]; then
        echo -e "${RED}No remote hosts configured${NC}"
        exit 1
    fi
    
    # Create temp directory for image tars
    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT
    
    # Save images to tar
    echo -e "${YELLOW}Saving images to tar files...${NC}"
    podman save catalogizer-api:latest -o "${TMP_DIR}/catalogizer-api.tar"
    podman save catalogizer-web:latest -o "${TMP_DIR}/catalogizer-web.tar"
    
    # Transfer to each host
    for host in ${hosts}; do
        echo -e "${YELLOW}Transferring to ${host}...${NC}"
        scp "${TMP_DIR}/catalogizer-api.tar" "${TMP_DIR}/catalogizer-web.tar" "${host}:/tmp/"
        
        echo -e "${YELLOW}Loading images on ${host}...${NC}"
        ssh "${host}" "podman load -i /tmp/catalogizer-api.tar && podman load -i /tmp/catalogizer-web.tar"
        
        echo -e "${GREEN}Images loaded on ${host}${NC}"
    done
}

start_services() {
    echo -e "${BLUE}=== Starting Services on Remote Hosts ===${NC}"
    
    hosts=$(get_remote_hosts)
    if [[ -z "${hosts}" ]]; then
        echo -e "${RED}No remote hosts configured${NC}"
        exit 1
    fi
    
    # Transfer docker-compose file
    for host in ${hosts}; do
        echo -e "${YELLOW}Setting up ${host}...${NC}"
        
        # Create directory
        ssh "${host}" "mkdir -p ~/catalogizer"
        
        # Transfer compose file
        scp "${PROJECT_ROOT}/docker-compose.dev.yml" "${host}:~/catalogizer/docker-compose.yml"
        
        # Create .env file for the services: random per-host secrets generated
        # locally (mode 0600, never printed) by the shared installer helper.
        _envdir=$(mktemp -d) || exit 1
        # Remove the temp env dir on ANY exit (generation failure included); the guarded
        # expansion makes an empty/unset value abort instead of expanding to a broad path.
        trap '[[ -n "${_envdir:-}" && -d "${_envdir}" ]] && rm -rf -- "${_envdir:?}"; [[ -n "${TMP_DIR:-}" && -d "${TMP_DIR}" ]] && rm -rf -- "${TMP_DIR:?}"; true' EXIT
        # shellcheck source=lib/install_env.sh
        . "${PROJECT_ROOT}/scripts/lib/install_env.sh"
        install_write_env "${_envdir}/.env" >/dev/null || exit 1
        printf 'DATABASE_TYPE=sqlite\nGIN_MODE=release\n' >> "${_envdir}/.env"
        scp -q "${_envdir}/.env" "${host}:~/catalogizer/.env"
        ssh "${host}" "chmod 600 ~/catalogizer/.env"
        rm -f "${_envdir:?}/.env"; rmdir "${_envdir:?}"; _envdir=""
        
        # Stop existing containers
        ssh "${host}" "cd ~/catalogizer && podman-compose down 2>/dev/null || true"
        
        # Start new containers
        echo -e "${YELLOW}Starting containers on ${host}...${NC}"
        ssh "${host}" "cd ~/catalogizer && podman-compose up -d"
        
        echo -e "${GREEN}Services started on ${host}${NC}"
    done
}

check_status() {
    echo -e "${BLUE}=== Service Status ===${NC}"
    
    hosts=$(get_remote_hosts)
    
    for host in ${hosts}; do
        echo -e "${YELLOW}Host: ${host}${NC}"
        ssh "${host}" "podman ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' | grep -E 'catalogizer|NAME' || echo 'No catalogizer containers running'"
        echo ""
    done
}

show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --build      Build container images"
    echo "  --transfer   Transfer images to remote hosts"
    echo "  --start      Start services on remote hosts"
    echo "  --status     Check service status"
    echo "  --all        Run all steps (build, transfer, start)"
    echo "  --help       Show this help"
    echo ""
    echo "Environment:"
    echo "  Configuration is read from: ${ENV_FILE}"
}

# Main
parse_env

if [[ $# -eq 0 ]]; then
    show_usage
    exit 0
fi

case "${1:-}" in
    --build)
        build_images
        ;;
    --transfer)
        transfer_images
        ;;
    --start)
        start_services
        ;;
    --status)
        check_status
        ;;
    --all)
        build_images
        transfer_images
        start_services
        check_status
        ;;
    --help|-h)
        show_usage
        ;;
    *)
        echo -e "${RED}Unknown option: ${1}${NC}"
        show_usage
        exit 1
        ;;
esac
