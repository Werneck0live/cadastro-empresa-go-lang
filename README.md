# API de Empresas (Golang + MongoDB + RabbitMQ)

## 

```

#go mod tidy

#RabbitMq http://localhost:15672 (guest/guest)


# Lei 8.213/91, art. 93:
#   100–200 funcionários → 2%
#   201–500 → 3%
#   501–1000 → 4%
#   1001+ → 5%
#   Empresas com <100 não têm exigência mínima.

# docker compose -f docker/docker-compose.yml down &&  sudo docker stop $(sudo docker ps -qa) && sudo docker rm $(sudo docker ps -qa) && sudo docker rmi $(sudo docker images -qa) && sudo docker compose -f docker/docker-compose.yml up -d

# docker compose -f docker/docker-compose.yml down && sudo docker rmi $(sudo docker images -qa) && docker compose -f docker/docker-compose.yml up -d --force-recreate

# --------------------- RABBIT ---------------------
# http://localhost:15672/#/queues

# --------------------- LOGS ---------------------
# docker compose -f docker/docker-compose.yml  logs -f api

# --------------------- TESTES --------------------- 
# Agora os testes rodam sem instalar pacotes a cada execução:
# docker compose  -f docker/docker-compose.yml  down && sudo docker compose up  -f docker/docker-compose.yml  -d
# docker compose -f docker/docker-compose.yml --profile test build ci --no-cache && docker compose -f docker/docker-compose.yml --profile test run --rm ci

# só unitários
# docker compose -f docker/docker-compose.yml --profile test run --rm -e RUN_INT=0 ci

# --------------------- SEEDER ---------------------
# docker compose -f docker/docker-compose.yml --profile admin build admin-seed --no-cache && docker compose -f docker/docker-compose.yml --profile admin run --rm admin-seed

# --------------------- WEB SOCKET ---------------------
# docker compose -f docker/docker-compose.yml up -d ws && docker compose -f docker/docker-compose.yml logs -f ws

# testar
# wscat -c ws://localhost:8090/ws
