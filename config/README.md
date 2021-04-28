## How to get this project set up on Cloud VPS:

On your local machine, wherever you keep your code:
```
git clone https://github.com/htried/wiki-diff-privacy.git
scp wiki-diff-privacy/config/*.sh <username>@diff-privacy-beam-test.wmf-research-tools.eqiad1.wikimedia.cloud:/home/<username>/

# these two are secret files, contact htriedman-ctr@wikimedia.org to access them
scp wiki-diff-privacy/config/replica.my.cnf <username>@diff-privacy-beam-test.wmf-research-tools.eqiad1.wikimedia.cloud:/home/<username>/
scp wiki-diff-privacy/config/config.sql <username>@diff-privacy-beam-test.wmf-research-tools.eqiad1.wikimedia.cloud:/home/<username>/
```

Now ssh into your Cloud VPS machine:
```
ssh <username>@diff-privacy-beam-test.wmf-research-tools.eqiad1.wikimedia.cloud
sudo bash cloudvps_setup.sh
sudo bash update_data.sh
```

At this point, the website should be working fine. You can check that by navigating to `diff-privacy-beam.wmcloud.org` on your browser.

Finally, set up a cron job to update the data every day at UTC+0900
```
sudo crontab -e
add a line reading “0 9 * * * /home/<username>/update_data.sh” to the end of the crontab
```

If you make changes to the codebase on your local machine, you can see those changes reflected in production by ssh-ing into your Cloud VPS machine and running `sudo bash release.sh`, which will pull from github and restart the nginx server with the relevant changes.