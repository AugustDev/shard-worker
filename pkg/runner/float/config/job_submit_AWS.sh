#!/bin/bash

# ---- User Configuration Section ----
# These configurations must be set by the user before running the script.

# ---- Optional Configuration Section ----
# These configurations are optional and can be customized as needed.

# JFS (JuiceFS) Private IP: Retrieved from the WORKER_ADDR environment variable.
jfs_private_ip=$(echo $WORKER_ADDR)

# Work Directory: Defines the root directory for working files. Optional suffix can be added.
workDir_suffix=''
workDir='/mnt/jfs/'$workDir_suffix
mkdir -p $workDir  # Ensures the working directory exists.
cd $workDir  # Changes to the working directory.
export NXF_HOME=$workDir  # Sets the NXF_HOME environment variable to the working directory.

# ---- Nextflow Configuration File Creation ----
# This section creates a Nextflow configuration file with various settings for the pipeline execution.

# Use cat to create or overwrite the mmc.config file with the desired Nextflow configurations.
# NOTE: S3 keys and OpCenter information will be concatted to the end of the config file. No need to add them now
cat > mmc.config << EOF
// enable nf-float plugin.
plugins {
    id 'nf-float'
}

SHARD_CONFIG_OVERRIDE

// Directories for Nextflow execution.
workDir = '${workDir}'
launchDir = '${workDir}'

EOF

# ---- Data Preparation ----
# Use this section to copy essential files from S3 to the working directory.

# For example, copy the sample sheet and params.yml from S3 to the current working directory.
# aws s3 cp s3://nextflow-input/samplesheet.csv .
# aws s3 cp s3://nextflow-input/scripts/params.yml .

# ---- Nextflow Command Setup ----
# Important: The -c option appends the mmc config file and soft overrides the nextflow configuration.

# Assembles the Nextflow command with all necessary options and parameters.
SHARD_NEXTFLOW_COMMAND

# -------------------------------------
# ---- DO NOT EDIT BELOW THIS LINE ----
# -------------------------------------
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

function remove_old_metadata () {
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
    echo $(date): "Previous metadata dump found! Removing $FOUND_METADATA"
    aws s3 rm $S3_MOUNT/$FOUND_METADATA
    echo $(date): "Previous metadata $FOUND_METADATA removed"
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

# Append to config file
cat <<EOT >> mmc.config

// OpCenter connection settings.
float {
    address = '${opcenter_ip_address}'
    username = '${opcenter_username}'
    password = '${opcenter_password}'
}

// AWS S3 Client configuration.
aws {
  client {
    maxConnections = 20
    connectionTimeout = 300000
  }
  accessKey = '${access_key}'
  secretKey = '${secret_key}'
}
EOT

# Create side script to tag head node - exits when properly tagged
cat > tag_nextflow_head.sh << EOF
#!/bin/bash

runname="\$(cat .nextflow.log 2>/dev/null | grep nextflow-io-run-name | head -n 1 | grep -oP '(?<=nextflow-io-run-name:)[^ ]+')"
workflowname="\$(cat .nextflow.log 2>/dev/null | grep nextflow-io-project-name | head -n 1 | grep -oP '(?<=nextflow-io-project-name:)[^ ]+')"

while true; do

  # Runname and workflowname will be populated at the same time
  # If the variables are populated and not tagged it, tag the head node
  if [ ! -z \$runname ]; then
    ./float modify -j "$(echo $FLOAT_JOB_ID)" --addCustomTag run-name:\$runname 2>/dev/null
    ./float modify -j "$(echo $FLOAT_JOB_ID)" --addCustomTag workflow-name:\$workflowname 2>/dev/null
    break
  fi

  runname="\$(cat .nextflow.log 2>/dev/null | grep nextflow-io-run-name | head -n 1 | grep -oP '(?<=nextflow-io-run-name:)[^ ]+')"
  workflowname="\$(cat .nextflow.log 2>/dev/null | grep nextflow-io-project-name | head -n 1 | grep -oP '(?<=nextflow-io-project-name:)[^ ]+')"

  sleep 1s

done
EOF

# Start tagging side-script
chmod +x ./tag_nextflow_head.sh
./tag_nextflow_head.sh &

# Start Nextflow run
$nextflow_command

if [[ $? -ne 0 ]]; then
  echo $(date): "Nextflow command failed."
  remove_old_metadata
  dump_and_cp_metadata
  copy_nextflow_log
  exit 1
else 
  echo $(date): "Nextflow command succeeded."
  remove_old_metadata
  dump_and_cp_metadata
  copy_nextflow_log
  exit 0
fi
