#!/bin/bash
# cd /var/www/example-app
# file="/var/www/example-app/composer.lock"
# if [ -f "$file" ]
# then
# 	composer update --prefer-source --no-interaction
# else
# 	composer install --prefer-source --no-interaction
# fi
# php /var/www/example-app/init --env=Development --overwrite=n
# php /var/www/example-app/yii migrate  --interactive=0
# php /var/www/example-app/yii rbac/init --interactive=0
# php /var/www/example-app/yii fixture/init --interactive=0
# php /var/www/example-app/yii search-index/init --interactive=0
# export PATH=$PATH:/var/www/example-app/vendor/bin
# cd  /var/www/example-app
# codecept build
# file="/var/www/example-app/vendor/npm/mosaico/dist/mosaico-material.min.css"
# if [ ! -f "$file" ]
# then
# 	wget -qO- https://deb.nodesource.com/setup_6.x | bash -
# 	apt-get install nodejs
# 	cd /var/www/example-app/vendor/npm/mosaico
# 	npm install --non-interactive
# 	npm install grunt grunt-cli --non-interactive
# 	ln -s /var/www/example-app/vendor/npm/mosaico/vendor/bower /var/www/example-app/vendor/npm/mosaico/bower_components
# 	grunt build --force
# fi
exec "$@"
