#!/usr/bin/env bash
# update API endpoint with new model, code, etc.

APP_LBL='diff-privacy-beam'  # descriptive label for endpoint-related directories
REPO_LBL='wiki-diff-privacy'  # directory where repo code will go
GIT_CLONE_HTTPS='https://github.com/htried/wiki-diff-privacy.git'  # for `git clone`
ETC_PATH="/etc/${APP_LBL}"  # app config info, scripts, ML models, etc.
SRV_PATH="/srv/${APP_LBL}"  # application resources for serving endpoint
TMP_PATH="/tmp/${APP_LBL}"  # store temporary files created as part of setting up app (cleared with every update)

# clean up old versions
rm -rf ${TMP_PATH}
mkdir -p ${TMP_PATH}

echo "Cloning repositories..."
git clone ${GIT_CLONE_HTTPS} ${TMP_PATH}/${REPO_LBL}

echo "Setting up Go dependencies and building binaries..."
cd ${TMP_PATH}/${REPO_LBL}
/usr/local/go/bin/go build -o ${SRV_PATH}/server server.go
/usr/local/go/bin/go build -o ${ETC_PATH}/resources/init_db init_db.go
/usr/local/go/bin/go build -o ${ETC_PATH}/resources/beam beam.go
/usr/local/go/bin/go build -o ${ETC_PATH}/resources/clean_db clean_db.go
cd

# update config / code -- if only changing Python and not nginx/uwsgi code, then much of this can be commented out
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