#!/usr/bin/env bash
# setup Cloud VPS instance with initial server etc.

# these can be changed but most other variables should be left alone
APP_LBL='diff-privacy-beam'  # descriptive label for endpoint-related directories
REPO_LBL='wiki-diff-privacy'  # directory where repo code will go
GIT_CLONE_HTTPS='https://github.com/htried/wiki-diff-privacy.git'  # for `git clone`

ETC_PATH="/etc/${APP_LBL}"  # app config info, scripts, ML models, etc.
SRV_PATH="/srv/${APP_LBL}"  # application resources for serving endpoint
TMP_PATH="/tmp/${APP_LBL}"  # store temporary files created as part of setting up app (cleared with every update)
LOG_PATH="/var/log/go"

echo "Updating the system..."
apt-get update
# apt-get install -y build-essential  # gcc (c++ compiler) necessary for fasttext
apt-get install -y nginx  # handles incoming requests, load balances, and passes to uWSGI to be fulfilled
apt-get install -y default-mysql-server

# apt-get install -y python3-pip  # install dependencies
# apt-get install -y python3-wheel  # make sure dependencies install correctly even when missing wheels
# apt-get install -y python3-venv  # for building virtualenv
# apt-get install -y python3-dev  # necessary for fasttext
# apt-get install -y uwsgi
# apt-get install -y uwsgi-plugin-python3
# potentially add: apt-get install -y git python3 libpython3.7 python3-setuptools

echo "Setting up Go..."
rm -rf /usr/local/go
tar -C /usr/local -xzf go1.16.3.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
export GOPATH=/usr/local/go


echo "Setting up paths..."
rm -rf ${TMP_PATH}
mkdir -p ${TMP_PATH}
mkdir -p ${SRV_PATH}/sock
mkdir -p ${ETC_PATH}
mkdir -p ${ETC_PATH}/resources


echo "Cloning repositories..."
git clone ${GIT_CLONE_HTTPS} ${TMP_PATH}/${REPO_LBL}

echo "Setting up Go dependencies and building binaries..."
cd ${TMP_PATH}/${REPO_LBL}
/usr/local/go/bin/go build -o ${SRV_PATH}/server server.go
/usr/local/go/bin/go build -o ${ETC_PATH}/resources/init_db init_db.go
/usr/local/go/bin/go build -o ${ETC_PATH}/resources/beam beam.go
/usr/local/go/bin/go build -o ${ETC_PATH}/resources/clean_db clean_db.go
cd

# If UI included, consider the following for managing JS dependencies:
# echo "Installing front-end resources..."
# mkdir -p ${SRV_PATH}/resources
# cd ${TMP_PATH}
# npm install bower
# cd ${SRV_PATH}/resources
# ${TMP_PATH}/node_modules/bower/bin/bower install --allow-root ${TMP_PATH}/recommendation-api/recommendation/web/static/bower.json

# echo "Downloading model, hang on..."
#cd ${TMP_PATH}
#wget -O model.bin ${MODEL_WGET}
#mv model.bin ${ETC_PATH}/resources

echo "Setting up ownership..."  # makes www-data (how nginx is run) owner + group for all data etc.
chown -R www-data:www-data ${ETC_PATH}
chown -R www-data:www-data ${SRV_PATH}

echo "Copying configuration files..."
cp ${TMP_PATH}/${REPO_LBL}/config/* ${ETC_PATH}
cp ${ETC_PATH}/app.nginx /etc/nginx/sites-available/app
if [[ -f "/etc/nginx/sites-enabled/app" ]]; then
    unlink /etc/nginx/sites-enabled/app
fi
ln -s /etc/nginx/sites-available/app /etc/nginx/sites-enabled/
cp ${ETC_PATH}/app.service /etc/systemd/system/

echo "Enabling and starting services..."
systemctl enable app.service  # uwsgi starts when server starts up
systemctl daemon-reload  # refresh state

systemctl restart app.service  # start up uwsgi
systemctl restart nginx  # start up nginx

service app start
nginx -s reload
