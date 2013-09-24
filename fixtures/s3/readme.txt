Synchronizes local content with remote S3 buckets.
Requires Ruby and S3Sync, both available via homebrew.

script synchronizes content from local test folder to remote buckets.
create a bucket:   


#example bucket creation and usage
s3cmd.rb createbucket buckets/fried.h0tb0x.com
echo Content > buckets/fried.h0tb0x.com/content.txt
./reset_buckets.sh
wget https://s3.amazonaws.com/fried.h0tb0x.com/content.txt

#example s3cmd manual file transfer
s3cmd.rb 
	put 
	caezar.h0tb0x.com:/caezars.stuff 
	buckets/caezar.h0tb0x.com/caezars.stuff 
	x-amz-acl:public-read