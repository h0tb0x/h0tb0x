#brew install ruby
#brew install s3sync

#tell s3sync where to find amazon certs
mkdir ~/.s3conf
mkdir ~/.s3conf/certs
cp ./s3config.yml ~/.s3conf/s3config.yml

#get amazon certs into place
cd ~/.s3conf/certs
wget http://mirbsd.mirsolutions.de/cvs.cgi/~checkout~/src/etc/ssl.certs.shar
sh ssl.certs.shar

echo complete
