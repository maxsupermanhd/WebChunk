#/bin/bash

read -p "Postgres will initialize in ./data, stop container when it will be done initializing (press enter to continue)"

docker-compose -f ./docker-compose-init.yml up

read -p "Postgres initialized, chown data dir to current user (press enter to continue)"

sudo chown -R "$(id -u):$(id -g)" ./data

echo "Init done, now you cna run the thing"
