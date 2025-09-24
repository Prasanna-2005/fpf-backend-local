#!/bin/sh

# Set proper ownership for log directory and shared storage
chown -R formpf:formpf /var/storage/formpf/ 

# Execute the command as the quantrium user
exec su formpf -c "$@"