#!/bin/bash

# Navigate to the repository root directory if started from within deployment/
if [ -f "../docker-compose.yml" ]; then
    cd ..
fi

echo "=========================================================="
echo "          Crystal Games Deployment Controller"
echo "=========================================================="
echo ""
echo "1. Start the services (docker compose up -d)"
echo "2. Stop the services (docker compose down)"
echo "3. Rebuild and Start (docker compose up -d --build)"
echo "4. View live logs (docker compose logs -f)"
echo "5. View status (docker compose ps)"
echo "6. Exit"
echo ""

read -p "Enter your choice (1-6): " choice

case $choice in
    1)
        echo "Starting services..."
        docker compose up -d
        ;;
    2)
        echo "Stopping services..."
        docker compose down
        ;;
    3)
        echo "Building and starting services..."
        docker compose up -d --build
        ;;
    4)
        echo "Streaming logs..."
        docker compose logs -f
        ;;
    5)
        echo "Service status:"
        docker compose ps
        ;;
    6)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo "Invalid option selected."
        ;;
esac
