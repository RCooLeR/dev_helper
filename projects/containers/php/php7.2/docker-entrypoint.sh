#!/bin/bash
# cd /var/www/datamarscrm
# file="/var/www/datamarscrm/composer.lock"
# if [ -f "$file" ]
# then
# 	composer update --prefer-source --no-interaction
# else
# 	composer install --prefer-source --no-interaction
# fi
# php /var/www/datamarscrm/init --env=Development --overwrite=n
# php /var/www/datamarscrm/yii migrate  --interactive=0
# php /var/www/datamarscrm/yii rbac/init --interactive=0
# php /var/www/datamarscrm/yii fixture/init --interactive=0
# php /var/www/datamarscrm/yii search-index/init --interactive=0
# export PATH=$PATH:/var/www/datamarscrm/vendor/bin
# cd  /var/www/datamarscrm
# codecept build
# file="/var/www/datamarscrm/vendor/npm/mosaico/dist/mosaico-material.min.css"
# if [ ! -f "$file" ]
# then
# 	wget -qO- https://deb.nodesource.com/setup_6.x | bash -
# 	apt-get install nodejs
# 	cd /var/www/datamarscrm/vendor/npm/mosaico
# 	npm install --non-interactive
# 	npm install grunt grunt-cli --non-interactive
# 	ln -s /var/www/datamarscrm/vendor/npm/mosaico/vendor/bower /var/www/datamarscrm/vendor/npm/mosaico/bower_components
# 	grunt build --force
# fi
exec "$@"
