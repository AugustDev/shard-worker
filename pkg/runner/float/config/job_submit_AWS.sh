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

# ------------------------------------------
# ---- vvv DO NOT EDIT THIS SECTION vvv ----
# ------------------------------------------

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

# Set Opcenter credentials
install_float 
access_key=$(get_secret AWS_BUCKET_ACCESS_KEY)
secret_key=$(get_secret AWS_BUCKET_SECRET_KEY)
export AWS_ACCESS_KEY_ID=$access_key
export AWS_SECRET_ACCESS_KEY=$secret_key

opcenter_ip_address=$(get_secret OPCENTER_IP_ADDRESS)
opcenter_username=$(get_secret OPCENTER_USERNAME)
opcenter_password=$(get_secret OPCENTER_PASSWORD)

# ------------------------------------------
# ---- ^^^ DO NOT EDIT THIS SECTION ^^^ ----
# ------------------------------------------

# ---- Nextflow Configuration File Creation ----
# This section creates a Nextflow configuration file with various settings for the pipeline execution.

# Use cat to create or overwrite the mmc.config file with the desired Nextflow configurations.
# NOTE: S3 keys and OpCenter information will be concatted to the end of the config file. No need to add them now

# Additionally, please add your STAGE MOUNT BUCKETS here
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

# ---- Data Preparation ----
# Use this section to copy essential files from S3 to the working directory.

# For example, copy the sample sheet and params.yml from S3 to the current working directory.
# aws s3 cp s3://nextflow-input/samplesheet.csv .
# aws s3 cp s3://nextflow-input/scripts/params.yml .

# ---- Nextflow Command Setup ----
# Important: The -c option appends the mmc config file and soft overrides the nextflow configuration.

# Assembles the Nextflow command with all necessary options and parameters.
SHARD_NEXTFLOW_COMMAND

# ---------------------------------------------
# ---- vvv DO NOT EDIT BELOW THIS LINE vvv ----
# ---------------------------------------------
# The following section contains functions and commands that should not be modified by the user.

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
  exit 1
else 
  echo $(date): "Nextflow command succeeded."
  exit 0
fi
