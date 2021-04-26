#!/usr/bin/env bash
# setup Cloud VPS instance with initial server etc.

# these can be changed but most other variables should be left alone
APP_LBL='diff-privacy-beam'  # descriptive label for endpoint-related directories
REPO_LBL='wiki-diff-privacy'  # directory where repo code will go
GIT_CLONE_HTTPS='https://github.com/htried/wiki-diff-privacy.git'  # for `git clone`
MODEL_WGET='https://ndownloader.figshare.com/files/<file-number>'  # model binary -- ndownloader.figshare is a good host

ETC_PATH="/etc/${APP_LBL}"  # app config info, scripts, ML models, etc.
SRV_PATH="/srv/${APP_LBL}"  # application resources for serving endpoint
TMP_PATH="/tmp/${APP_LBL}"  # store temporary files created as part of setting up app (cleared with every update)
LOG_PATH="/var/log/go"  # application log data
LIB_PATH="/var/lib/${APP_LBL}"  # where virtualenv will sit

echo "Updating the system..."
apt-get update
# apt-get install -y build-essential  # gcc (c++ compiler) necessary for fasttext
apt-get install -y nginx  # handles incoming requests, load balances, and passes to uWSGI to be fulfilled
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
mkdir -p ${LOG_PATH}
mkdir -p ${LIB_PATH}


echo "Cloning repositories..."
# NOTE: a more stable install would involve building wheels on an identical instance and then the following:
# NOTE: see (https://gerrit.wikimedia.org/g/research/recommendation-api/wheels/+/refs/heads/master) for an example.
# git clone https://gerrit.wikimedia.org/r/research/recommendation-api/wheels ${TMP_PATH}/wheels
# echo "Making wheel files..."
# cd ${TMP_PATH}/wheels
# rm -rf wheels/*.whl
# make
# git clone ${GIT_CLONE_HTTPS} ${TMP_PATH}/${REPO_LBL}
# echo "Installing repositories..."
# pip3 install --no-deps ${TMP_PATH}/wheels/wheels/*.whl
# pip3 install --no-deps ${TMP_PATH}/recommendation-api

# The simpler process is to just install dependencies per a requirements.txt file
# With updates, however, the packages could change, leading to unexpected behavior or errors
git clone ${GIT_CLONE_HTTPS} ${TMP_PATH}/${REPO_LBL}

echo "Setting up Go dependencies..."
cd ${TMP_PATH}/${REPO_LBL}
go build server.go
go build init_db.go
go build beam.go
go build clean_db.go

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
chown -R www-data:www-data ${LOG_PATH}
chown -R www-data:www-data ${LIB_PATH}

echo "Copying configuration files..."
cp ${TMP_PATH}/${REPO_LBL}/model/config/* ${ETC_PATH}
# TODO: fix this to be more elegant (one directory or not necessary because run as package)
# cp ${TMP_PATH}/${REPO_LBL}/model/wsgi.py ${ETC_PATH}
# cp ${TMP_PATH}/${REPO_LBL}/model/flask_config.yaml ${ETC_PATH}
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