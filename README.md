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

#sudo docker stop $(sudo docker ps -qa) && sudo docker rm $(sudo docker ps -qa) && sudo docker rmi $(sudo docker images -qa) && sudo docker compose -f docker-compose.yml up -d