#!/bin/bash
#forces state of several S3 buckets using s3sync

#should probably load different creds per bucket, using caezar's now
export AWS_ACCESS_KEY_ID=AKIAITK3WQM7ZZJV5EFQ
export AWS_SECRET_ACCESS_KEY=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

for BUCKET in buckets/*; do
	if [ -d "$BUCKET" ]; then
		s3sync.rb --public-read -v $BUCKET/ $(basename $BUCKET):/
	fi
done
