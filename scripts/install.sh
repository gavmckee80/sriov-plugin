#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="sriov-manager"
BINARY_NAME="sriov-manager"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/sriov-manager"
SERVICE_FILE="/etc/systemd/system/sriov-manager.service"

echo -e "${GREEN}Installing SR-IOV Manager Service${NC}"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root${NC}"
   exit 1
fi

# Build the binary
echo -e "${YELLOW}Building SR-IOV Manager...${NC}"
if ! go build -o bin/${BINARY_NAME} cmd/sriov-manager/main.go; then
    echo -e "${RED}Failed to build SR-IOV Manager${NC}"
    exit 1
fi

# Create installation directory
echo -e "${YELLOW}Creating installation directories...${NC}"
mkdir -p ${INSTALL_DIR}
mkdir -p ${CONFIG_DIR}

# Install binary
echo -e "${YELLOW}Installing binary...${NC}"
cp bin/${BINARY_NAME} ${INSTALL_DIR}/
chmod +x ${INSTALL_DIR}/${BINARY_NAME}

# Install service file
echo -e "${YELLOW}Installing systemd service...${NC}"
cp scripts/sriov-manager.service ${SERVICE_FILE}

# Create default configuration if it doesn't exist
if [[ ! -f "${CONFIG_DIR}/config.json" ]]; then
    echo -e "${YELLOW}Creating default configuration...${NC}"
    ${INSTALL_DIR}/${BINARY_NAME} --create-config
fi

# Reload systemd
echo -e "${YELLOW}Reloading systemd...${NC}"
systemctl daemon-reload

# Enable service
echo -e "${YELLOW}Enabling service...${NC}"
systemctl enable ${SERVICE_NAME}

echo -e "${GREEN}Installation completed successfully!${NC}"
echo -e "${YELLOW}Next steps:${NC}"
echo -e "  1. Review configuration: ${CONFIG_DIR}/config.json"
echo -e "  2. Test discovery: ${INSTALL_DIR}/${BINARY_NAME} --discover"
echo -e "  3. Validate configuration: ${INSTALL_DIR}/${BINARY_NAME} --validate"
echo -e "  4. Start service: systemctl start ${SERVICE_NAME}"
echo -e "  5. Check status: systemctl status ${SERVICE_NAME}" 