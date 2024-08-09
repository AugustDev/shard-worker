#!/bin/bash

# Cd into correct directory
export HOME=/root
cd /mnt/jfs
echo $HOME

# The following section contains functions and commands that should not be modified by the user.

function install_float {
  # Install float
  local address=$(echo "$FLOAT_ADDR" | cut -d':' -f1)
  wget https://$address/float --no-check-certificate --quiet
  chmod +x float
}

function get_secret {
  input_string=$1
  local address=$(echo "$FLOAT_ADDR" | cut -d':' -f1)
  secret_value=$(./float secret get $input_string -a $address)

  if [[ $? -eq 0 ]]; then
    # Have this secret, will use the secret value
    echo $secret_value
    return
  else
    # Don't have this secret, will still use the input string
    echo $1
  fi
}

function aws_keys() {
  local access_key=$(get_secret AWS_BUCKET_ACCESS_KEY)
  local secret_key=$(get_secret AWS_BUCKET_SECRET_KEY)
  export AWS_ACCESS_KEY_ID=$access_key
  export AWS_SECRET_ACCESS_KEY=$secret_key
}

function find_old_metadata () {
  echo $(date): "First finding and removing old metadata..."
  if [[ $BUCKET == *"amazonaws.com"* ]]; then
    # If default `amazonaws.com` endpoint url
    S3_MOUNT=s3://$(echo $BUCKET | sed 's:.*/::' | awk -F'[/.]s3.' '{print $1}')
  else
    # If no 'amazonaws.com,' the bucket is using a custom endpoint
    local bucket_name=$(echo $BUCKET | sed 's:.*/::' | awk -F'[/.]s3.' '{print $1}')
    S3_MOUNT="--endpoint-url $(echo "${BUCKET//$bucket_name.}") s3://$bucket_name"
  fi
  # If a previous job id was given, we use that as the old metadata
  if [[ ! -z $PREVIOUS_JOB_ID ]]; then
    echo $(date): "Previous job id $PREVIOUS_JOB_ID specified. Looking for metadata file in bucket..."
    FOUND_METADATA=$(aws s3 ls $S3_MOUNT | grep "$PREVIOUS_JOB_ID.meta.json.gz" | awk '{print $4}')
  fi

  if [[ -z "$FOUND_METADATA" ]]; then
    # If no previous job id was given, there is no old metadata to remove.
    echo $(date): "No previous metadata dump found. Continuing with dumping current JuiceFs"
  else
    echo $(date): "Previous metadata dump found!"
  fi

}

function dump_and_cp_metadata() {
  echo $(date): "Attempting to dump JuiceFS data"

  if [[ -z "$FOUND_METADATA" ]]; then
    # If no previous metadata was found, use the current job id
    juicefs dump redis://$(echo $WORKER_ADDR):6868/1 $(echo $FLOAT_JOB_ID).meta.json.gz --keep-secret-key
    echo $(date): "JuiceFS metadata $FLOAT_JOB_ID.meta.json.gz created. Copying to JuiceFS Bucket"
    aws s3 cp "$(echo $FLOAT_JOB_ID).meta.json.gz" $S3_MOUNT
  else
    # If previous metadata was found, use the id of the previous metadata
    # This means for all jobs that use the same mount, their id will always be their first job id
    metadata_name=$PREVIOUS_JOB_ID
    juicefs dump redis://$(echo $WORKER_ADDR):6868/1 $(echo $metadata_name).meta.json.gz --keep-secret-key
    echo $(date): "JuiceFS metadata $metadata_name.meta.json.gz created. Copying to JuiceFS Bucket"
    aws s3 cp "$(echo $metadata_name).meta.json.gz" $S3_MOUNT
  fi

  echo $(date): "Copying to JuiceFS Bucket complete!"
}

function copy_nextflow_log() {
  echo $(date): "Copying .nextflow.log to bucket.."
  if [[ ! -z $PREVIOUS_JOB_ID ]]; then
    aws s3 cp ".nextflow.log" $S3_MOUNT/$PREVIOUS_JOB_ID.nextflow.log
    echo $(date): "Copying .nextflow.log complete! You can find it with aws s3 ls $S3_MOUNT/$PREVIOUS_JOB_ID.nextflow.log"
  else
    aws s3 cp ".nextflow.log" $S3_MOUNT/$(echo $FLOAT_JOB_ID).nextflow.log
    echo $(date): "Copying .nextflow.log complete! You can find it with aws s3 ls $S3_MOUNT/$(echo $FLOAT_JOB_ID).nextflow.log"
  fi
}

# Variables
S3_MOUNT=""
FOUND_METADATA=""

# Functions pre-Nextflow run
# AWS S3 Access and Secret Keys: For accessing S3 buckets.
install_float 
access_key=$(get_secret AWS_BUCKET_ACCESS_KEY)
secret_key=$(get_secret AWS_BUCKET_SECRET_KEY)
export AWS_ACCESS_KEY_ID=$access_key
export AWS_SECRET_ACCESS_KEY=$secret_key

opcenter_ip_address=$(get_secret OPCENTER_IP_ADDRESS)
opcenter_username=$(get_secret OPCENTER_USERNAME)
opcenter_password=$(get_secret OPCENTER_PASSWORD)

find_old_metadata
dump_and_cp_metadata
copy_nextflow_log
