#!/bin/bash
# File: /scripts/dev.sh

# Development script for MotoCosmos API

case "$1" in
  "start")
    echo "🚀 Starting development environment..."
    docker-compose -f docker-compose.dev.yml up -d --build
    echo "✅ Development environment started!"
    echo "📊 API: http://localhost:8089"
    echo "🗄️  Database: localhost:3310"
    echo "📡 Redis: localhost:6380"
    echo ""
    echo "📝 Logs: docker-compose -f docker-compose.dev.yml logs -f api"
    ;;
  "stop")
    echo "🛑 Stopping development environment..."
    docker-compose -f docker-compose.dev.yml down
    echo "✅ Development environment stopped!"
    ;;
  "restart")
    echo "🔄 Restarting development environment..."
    docker-compose -f docker-compose.dev.yml restart api
    echo "✅ API restarted!"
    ;;
  "logs")
    echo "📝 Showing API logs..."
    docker-compose -f docker-compose.dev.yml logs -f api
    ;;
  "db")
    echo "🗄️ Connecting to database..."
    docker exec -it motocosmos_db_dev mysql -u motocosmos_user -pmotocosmos_password motocosmos
    ;;
  "clean")
    echo "🧹 Cleaning up development environment..."
    docker-compose -f docker-compose.dev.yml down -v
    docker system prune -f
    echo "✅ Cleanup complete!"
    ;;
  "build")
    echo "🔨 Rebuilding development environment..."
    docker-compose -f docker-compose.dev.yml up -d --build --force-recreate
    echo "✅ Rebuild complete!"
    ;;
  *)
    echo "🏍️  MotoCosmos Development Helper"
    echo ""
    echo "Usage: $0 {start|stop|restart|logs|db|clean|build}"
    echo ""
    echo "Commands:"
    echo "  start    - Start development environment with hot reload"
    echo "  stop     - Stop development environment"
    echo "  restart  - Restart API container only"
    echo "  logs     - Show API logs"
    echo "  db       - Connect to development database"
    echo "  clean    - Stop and remove all containers, networks, and volumes"
    echo "  build    - Force rebuild all containers"
    echo ""
    echo "🔥 Hot reload is enabled - your code changes will be automatically applied!"
    ;;
esac