
## Docker eclipse mosquitto
```bash
#!/bin/bash
FAPP=eclipse-mosquitto-latest
docker stop $FAPP
docker rm $FAPP
docker pull eclipse-mosquitto:latest
PWD=`pwd`
docker run --name=$FAPP -d --restart=always --net=host -e TZ=Europe/Moscow -v "$PWD/config:/mosquitto/config" -v "$PWD/data:/mosquitto/data" -v "$PWD/log:/mosquitto/log" eclipse-mosquitto:latest
docker logs -f $FAPP
```

## Config eclipse mosquitto
```conf
autosave_interval 1800
autosave_on_changes false
persistence true
persistence_location /mosquitto/data/
per_listener_settings false
#per_listener_settings true
#log_type debug
log_dest file /mosquitto/log/mosquitto.log

listener 1883

acl_file /mosquitto/config/acl.conf

allow_anonymous false
password_file /mosquitto/config/password.conf

log_timestamp_format %Y-%m-%dT%H:%M:%S
```

## Create a user/password in the pwfile
```conf
# login interactively into the mqtt container
sudo docker exec -it <container-id> sh

# Create new password file and add user and it will prompt for password
mosquitto_passwd -c /mosquitto/config/pwfile user1

# Add additional users (remove the -c option) and it will prompt for password
mosquitto_passwd /mosquitto/config/pwfile user2

# delete user command format
mosquitto_passwd -D /mosquitto/config/pwfile <user-name-to-delete>

# exit out of docker container ctrl + p & ctrl + q
```
