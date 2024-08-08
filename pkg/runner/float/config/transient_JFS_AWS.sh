#!/bin/bash
#
# This script will be submitted as a contianerInit hook script and do following things:
#
# - Setup a temporary JuiceFS filesystem as the working directory.
# - Install nextflow software and its dependencies on the working directory.
# - Generate the nextflow configuration file for MMCloud.
# - Setup the nextflow working directory and environment.
#
# All the needed parameters will be passed in as environment variables.
#
# BUCKET: The bucket for JuiceFS storage.
# AWS_BUCKET_ACCESS_KEY: The bucket access key for JuiceFS. (Optional)
# AWS_BUCKET_SECRET_KEY: The bucket secret key for JuiceFS. (Optional)
# JFS_CACHE_DIR: The cache directory for JuiceFS. (default is /mnt/jfs_cache)
# JFS_MOUNT_POINT: The mount point for JuiceFS. (default is /mnt/jfs)
# JFS_MOUNT_OPTS: The mount options of JuiceFS.

#set -x
export PATH=$PATH:/usr/bin:/usr/local/bin:/opt/memverge/bin
export HOME=/root
BUCKET=${BUCKET:-""}
PREVIOUS_JOB_ID=${PREVIOUS_JOB_ID:-""}
AWS_BUCKET_ACCESS_KEY=${AWS_BUCKET_ACCESS_KEY:-""}
AWS_BUCKET_SECRET_KEY=${AWS_BUCKET_SECRET_KEY:-""}
FLOAT_VMPOLICY=${FLOAT_VMPOLICY:-""}
NF_FLOAT_VERSION=${NF_FLOAT_VERSION:-""}
JFS_META_PORT=${JFS_META_PORT:-6868}
JFS_MOUNT_POINT=${JFS_MOUNT_POINT:-/mnt/jfs}
JFS_CACHE_DIR=${JFS_CACHE_DIR:-/mnt/jfs_cache}
JFS_CACHE_SIZE=${JFS_CACHE_SIZE:-120}
JFS_MOUNT_OPTS=${JFS_MOUNT_OPTS:-""}
JFS_MOUNT_OPTS=" --cache-dir $JFS_CACHE_DIR $JFS_MOUNT_OPTS"
FOUND_METADATA=""

LOG_FILE=$FLOAT_JOB_PATH/container-init.log
touch $LOG_FILE
exec >$LOG_FILE 2>&1

function log() {
  if [[ -f ${LOG_FILE_PATH} ]]; then
    echo $(date): "$@" >>${LOG_FILE_PATH}
  fi
  echo $(date): "$@"
}

function error() {
  log "[ERROR] $1"
}

function die() {
  error "$1"
  podman kill -a 2>&1 >/dev/null
  exit 1
}

function trim_quotes() {
  : "${1//\'/}"
  printf '%s\n' "${_//\"/}"
}

function assure_root() {
  if [[ ${EUID} -ne 0 ]]; then
    die "Please run with root or sudo privilege."
  fi
}

function echolower {
  tr [:upper:] [:lower:] <<<"${*}"
}

function get_secret {
  input_string=$1
  secret_value=$(float secret get $input_string -a $FLOAT_ADDR)
  if [[ $? -eq 0 ]]; then
    # Have this secret, will use the secret value
    echo $secret_value
    return
  else
    # Don't have this secret, will still use the input string
    echo $1
  fi
}

function set_secret {
  file_name=$1
  secret_name=${FLOAT_JOB_ID}_SSHKEY
  float secret set $secret_name --file $file_name -a $FLOAT_ADDR
  if [[ $? -ne 0 ]]; then
    die "Set secret $secret_name failed"
  fi
}

function prepare_redis_cli() {
  redis_cli_path=$(which redis-cli)
  if [[ $? -eq 0 ]]; then
    log "Redis-cli is already installed at $redis_cli_path"
    return
  fi
  log "Install Redis-cli"
  yum install -y --quiet redis
  if [[ $? -ne 0 ]]; then
    die "Install Redis-cli failed"
  fi
}

function start_redis_server() {
  # Make redis.conf file
  echo -n "bind 0.0.0.0 -::1
port 6868
daemonize yes
#requirepass mmcloud
maxmemory-policy noeviction
save ""
appendonly no
" > /etc/redis.conf
  redis-server /etc/redis.conf
}

function prepare_juicefs() {
  juicefs_cli_path=$(which juicefs)
  if [[ $? -eq 0 ]]; then
    log "juicefs-cli is already installed at $juicefs_cli_path"
    return
  fi
  log "Install JuiceFS"
  curl -sSL https://d.juicefs.com/install | sh -
  if [[ $? -ne 0 ]]; then
    die "Install JuiceFS failed"
  fi
}

function aws_keys() {
  local access_key=$(get_secret AWS_BUCKET_ACCESS_KEY)
  local secret_key=$(get_secret AWS_BUCKET_SECRET_KEY)
  export AWS_ACCESS_KEY_ID=$access_key
  export AWS_SECRET_ACCESS_KEY=$secret_key
}

function load_juicefs_meta() {
  if [[ $BUCKET == *"amazonaws.com"* ]]; then
    # If default `amazonaws.com` endpoint url
    S3_MOUNT=s3://$(echo $BUCKET | sed 's:.*/::' | awk -F'[/.]s3.' '{print $1}')
  else
    # If no 'amazonaws.com,' the bucket is using a custom endpoint
    local bucket_name=$(echo $BUCKET | sed 's:.*/::' | awk -F'[/.]s3.' '{print $1}')
    # Format is --endpoint-url https://s3.endpoint.url s3://bucket_name
    S3_MOUNT="--endpoint-url $(echo "${BUCKET//$bucket_name.}") s3://$bucket_name"
  fi

  # If a previous jobid is specified, we will use that as the metadata id
  if [[ ! -z $PREVIOUS_JOB_ID ]]; then
    log "Previous job id $PREVIOUS_JOB_ID specified. Looking for metadata file in bucket..."
    FOUND_METADATA=$(aws s3 ls $S3_MOUNT | grep "$PREVIOUS_JOB_ID.meta.json.gz" | awk '{print $4}')
    
    if [[ -z "$FOUND_METADATA" ]]; then
      log "Specified metadata id $PREVIOUS_JOB_ID NOT found. Continuing with regular Juicefs format."
    else
      log "Metadata $FOUND_METADATA found! Looking for matching JuiceFS mount"
      local find_juicefs_name=$(echo $FOUND_METADATA | awk -F '.meta.json.gz' '{print $1}')
      local found_juicefs_dir=$(aws s3 ls $S3_MOUNT | grep $(echo $find_juicefs_name) | grep -v ".meta.json.gz" | awk '{print $2}')

      if [[ -z "$found_juicefs_dir" ]]; then
        die "No matching JuiceFS folder of name $find_juicefs_name. Please remove $FOUND_METADATA from bucket $S3_MOUNT and retry."
      else
        log "Metadata and matching JuiceFS found! Copying $FOUND_METADATA"
        aws s3 cp $S3_MOUNT/$FOUND_METADATA .
        log "Loading $FOUND_METADATA"
        log "juicefs load redis://127.0.0.1:$JFS_META_PORT/1 $FOUND_METADATA"
        juicefs load redis://127.0.0.1:$JFS_META_PORT/1 $FOUND_METADATA
      fi
    fi 
  # If no previous jobid specified, continue with making a NEW mount in format_jfs()
  fi
}

function format_jfs() {
  AWS_BUCKET_ACCESS_KEY_OPT=""
  AWS_BUCKET_SECRET_KEY_OPT=""
  if [[ ! -z $AWS_ACCESS_KEY_ID ]]; then
    AWS_BUCKET_ACCESS_KEY_VALUE=$(get_secret AWS_BUCKET_ACCESS_KEY)
    export AWS_BUCKET_ACCESS_KEY_OPT=" --access-key $AWS_BUCKET_ACCESS_KEY_VALUE"
  fi
  if [[ ! -z AWS_SECRET_ACCESS_KEY ]]; then
    AWS_BUCKET_SECRET_KEY_VALUE=$(get_secret AWS_BUCKET_SECRET_KEY)
    export AWS_BUCKET_SECRET_KEY_OPT=" --secret-key $AWS_BUCKET_SECRET_KEY_VALUE"
  fi

  if [[ -z "$FOUND_METADATA" ]]; then
    # FOUND_METADATA is only populated if a previously metadata (when given previous job id) is found
    # If no previous metadata is found (because no job id given), we can set name to job name
    fs_name=$(echolower $FLOAT_JOB_ID)
    log "Formatting new JuiceFS"
    log "juicefs format --storage s3 --bucket $BUCKET redis://127.0.0.1:$JFS_META_PORT/1 $fs_name AWS_BUCKET_ACCESS_KEY AWS_BUCKET_SECRET_KEY --trash-days=0"
    juicefs format --storage s3 --bucket $BUCKET redis://127.0.0.1:$JFS_META_PORT/1 $fs_name $AWS_BUCKET_ACCESS_KEY_OPT $AWS_BUCKET_SECRET_KEY_OPT --trash-days=0 2>&1 >/dev/null
  else
    # If previous metadata found (when given proper previous job id), we need to use the same name
    metadata_name=$PREVIOUS_JOB_ID
    log "Configurating existing $metadata_name JuiceFS"
    log "juicefs config --yes --storage s3 --bucket $BUCKET redis://127.0.0.1:$JFS_META_PORT/1 $metadata_name AWS_BUCKET_ACCESS_KEY AWS_BUCKET_SECRET_KEY --trash-days=0"
    juicefs config --yes --storage s3 --bucket $BUCKET redis://127.0.0.1:$JFS_META_PORT/1 $metadata_name $AWS_BUCKET_ACCESS_KEY_OPT $AWS_BUCKET_SECRET_KEY_OPT --trash-days=0 2>&1 >/dev/null
  fi
  if [[ $? -ne 0 ]]; then
      die "Format JuiceFS failed"
    fi
}

function mount_jfs {
  jfs_mnt=$(mount | grep $JFS_MOUNT_POINT | awk -F : '{print $1}')
  if [[ "$jfs_mnt" == "JuiceFS" ]]; then
    log "JuiceFS is already mounted at $JFS_MOUNT_POINT"
    return
  fi
  log "Mount JuiceFS"
  mkdir -p $JFS_CACHE_DIR
  mkdir -p /mnt/jfs
  chmod 777 /mnt/jfs
  log "juicefs mount redis://127.0.0.1:$JFS_META_PORT/1 $JFS_MOUNT_POINT -d $JFS_MOUNT_OPTS --root-squash $FLOAT_USER_ID"
  juicefs mount redis://127.0.0.1:$JFS_META_PORT/1 $JFS_MOUNT_POINT -d $JFS_MOUNT_OPTS --root-squash $FLOAT_USER_ID --log $FLOAT_INSTANCE_FOLDER/jfs.log
}

function prepare_user_env {
  if [[ $FLOAT_USER_ID -eq 0 ]]; then
    USER_PROFILE=/root/.bash_profile
  else
    systemctl stop munge
    /usr/sbin/userdel slurm
    /usr/sbin/userdel munge
    USER_HOME=/home/$FLOAT_USER
    /usr/sbin/useradd -u $FLOAT_USER_ID -d $USER_HOME -s /bin/bash $FLOAT_USER
    USER_PROFILE=$USER_HOME/.bash_profile
  fi
}

function check_args {
  if [[ -z "$BUCKET" ]]; then
    die "BUCKET is not set"
  fi
  BUCKET=$(trim_quotes "$BUCKET")
}

#env

# Execute the script
check_args
assure_root
prepare_redis_cli
start_redis_server
aws_keys
prepare_juicefs
load_juicefs_meta
format_jfs
mount_jfs
prepare_user_env

exit 0
