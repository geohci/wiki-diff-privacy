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

echo "Downloading and configuring server and database..."
apt-get install -y nginx  # handles incoming requests, load balances, and passes to uWSGI to be fulfilled
apt-get install -y default-mysql-server # handles data storage
mysql < config.sql # set administrator profile to have privileges

echo "Setting up Go..."
rm -rf /usr/local/go
cd /tmp
wget https://golang.org/dl/go1.16.3.linux-amd64.tar.gz # download go
tar -C /usr/local -xzf go1.16.3.linux-amd64.tar.gz # untar and install
rm go1.16.3.linux-amd64.tar.gz
cd
# set params in .profile
echo "export GOROOT=/usr/local/go" >> ~/.profile
echo "export GOPATH=$HOME/go" >> ~/.profile
echo "export PATH=$GOPATH/bin:$GOROOT/bin:$PATH" >> ~/.profile
source ~/.profile

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

echo "Setting up ownership..."  # makes www-data (how nginx is run) owner + group for all data etc.
chown -R www-data:www-data ${ETC_PATH}
chown -R www-data:www-data ${SRV_PATH}

echo "Copying static files..." # copies static assets to /etc/ where they'll be accessible
cp ${TMP_PATH}/${REPO_LBL}/static/* ${ETC_PATH}
cp ${TMP_PATH}/${REPO_LBL}/templates/* ${ETC_PATH}

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
