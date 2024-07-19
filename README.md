# Shard Worker

Shard worker executed Nextflow and Float executor workflows from nf-shard.

## Deployment instructions

### AWS Fargate

```sh
aws ecs create-cluster --cluster-name shard-worker-cluster

aws ecs register-task-definition --cli-input-json file://task-definition.json

aws ecs create-service \
    --cluster shard-worker-cluster \
    --service-name shard-worker-service \
    --task-definition shard-worker-task \
    --desired-count 1 \
    --launch-type FARGATE \
    --network-configuration "awsvpcConfiguration={subnets=[subnet-11111],securityGroups=[sg-11111],assignPublicIp=ENABLED}"

aws ecs update-service \
    --cluster shard-worker-cluster \
    --service shard-worker-service \
    --task-definition shard-worker-task

```

### Terraform

Coming soon
