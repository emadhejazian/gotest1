# AWS Deployment Guide - Step by Step

This guide will help you deploy your Go API to AWS using ECS (Elastic Container Service) with Fargate.

## Architecture Overview

- **Amazon RDS**: Managed PostgreSQL database
- **Amazon ECR**: Docker image registry
- **Amazon ECS**: Container orchestration (with Fargate for serverless containers)
- **Application Load Balancer**: Routes traffic to your containers

## Prerequisites

### 1. Create an AWS Account

1. Go to [aws.amazon.com](https://aws.amazon.com)
2. Click "Create an AWS Account"
3. Follow the registration process (requires credit card, but we'll use free tier services)
4. Verify your email and phone number

### 2. Install AWS CLI

**For macOS:**
```bash
# Install AWS CLI v2
curl "https://awscli.amazonaws.com/AWSCLIV2.pkg" -o "AWSCLIV2.pkg"
sudo installer -pkg AWSCLIV2.pkg -target /

# Verify installation
aws --version
```

### 3. Create AWS Access Keys

1. Log in to [AWS Console](https://console.aws.amazon.com)
2. Click your username (top right) → Security credentials
3. Scroll to "Access keys" → Create access key
4. Choose "Command Line Interface (CLI)"
5. Download the key file (keep it secure!)

### 4. Configure AWS CLI

```bash
aws configure
```

Enter when prompted:
- **AWS Access Key ID**: [from step 3]
- **AWS Secret Access Key**: [from step 3]
- **Default region name**: `us-east-1` (or your preferred region)
- **Default output format**: `json`

---

## Step 1: Create PostgreSQL Database on RDS

### Option A: Using AWS Console (Recommended for beginners)

1. Go to [RDS Console](https://console.aws.amazon.com/rds)
2. Click "Create database"
3. Choose:
   - **Engine**: PostgreSQL
   - **Templates**: Free tier
   - **DB instance identifier**: `gotest1-db`
   - **Master username**: `gotest1`
   - **Master password**: Create a strong password (save it!)
   - **DB instance class**: `db.t3.micro` (free tier)
   - **Storage**: 20 GB
   - **Public access**: Yes (for now, we'll secure it later)
   - **Initial database name**: `gotest1`
4. Click "Create database"
5. Wait 5-10 minutes for creation
6. Note the **Endpoint** (looks like: `gotest1-db.xxxxx.us-east-1.rds.amazonaws.com`)

### Option B: Using AWS CLI

```bash
# Create RDS PostgreSQL instance
aws rds create-db-instance \
  --db-instance-identifier gotest1-db \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --engine-version 15.5 \
  --master-username gotest1 \
  --master-user-password YOUR_STRONG_PASSWORD \
  --allocated-storage 20 \
  --db-name gotest1 \
  --publicly-accessible \
  --backup-retention-period 7

# Check status (wait until "available")
aws rds describe-db-instances \
  --db-instance-identifier gotest1-db \
  --query 'DBInstances[0].DBInstanceStatus'

# Get endpoint
aws rds describe-db-instances \
  --db-instance-identifier gotest1-db \
  --query 'DBInstances[0].Endpoint.Address' \
  --output text
```

**Save your database endpoint!** You'll need it later.

---

## Step 2: Run Database Migration

Once your RDS database is available, run the migration:

```bash
# Replace with your actual RDS endpoint and password
export DB_ENDPOINT="gotest1-db.xxxxx.us-east-1.rds.amazonaws.com"
export DB_PASSWORD="your_password"

# Install PostgreSQL client if not already installed
brew install postgresql

# Run migration
PGPASSWORD=$DB_PASSWORD psql \
  -h $DB_ENDPOINT \
  -U gotest1 \
  -d gotest1 \
  -f db/migrations/001_create_users.sql

# Verify table was created
PGPASSWORD=$DB_PASSWORD psql \
  -h $DB_ENDPOINT \
  -U gotest1 \
  -d gotest1 \
  -c "\dt"
```

---

## Step 3: Create ECR Repository and Push Docker Image

### Create ECR Repository

```bash
# Create repository
aws ecr create-repository \
  --repository-name gotest1 \
  --region us-east-1

# Get repository URI (save this!)
aws ecr describe-repositories \
  --repository-names gotest1 \
  --query 'repositories[0].repositoryUri' \
  --output text
```

### Login to ECR

```bash
# Get login password and authenticate Docker
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin $(aws sts get-caller-identity --query Account --output text).dkr.ecr.us-east-1.amazonaws.com
```

### Build and Push Image

```bash
# Get your AWS account ID
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

# Build image for production (without local dependencies)
docker build -t gotest1 .

# Tag image
docker tag gotest1:latest $AWS_ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/gotest1:latest

# Push to ECR
docker push $AWS_ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/gotest1:latest
```

---

## Step 4: Create ECS Cluster

```bash
# Create ECS cluster (Fargate)
aws ecs create-cluster \
  --cluster-name gotest1-cluster \
  --region us-east-1

# Verify
aws ecs describe-clusters \
  --clusters gotest1-cluster \
  --query 'clusters[0].status'
```

---

## Step 5: Create Task Execution Role

This role allows ECS to pull images from ECR and write logs to CloudWatch.

```bash
# Create trust policy file
cat > ecs-task-trust-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF

# Create role
aws iam create-role \
  --role-name ecsTaskExecutionRole \
  --assume-role-policy-document file://ecs-task-trust-policy.json

# Attach AWS managed policy
aws iam attach-role-policy \
  --role-name ecsTaskExecutionRole \
  --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy

# Get role ARN (save this!)
aws iam get-role \
  --role-name ecsTaskExecutionRole \
  --query 'Role.Arn' \
  --output text
```

---

## Step 6: Create ECS Task Definition

Create a file with your task definition:

```bash
# Replace these values:
# - YOUR_ACCOUNT_ID
# - YOUR_DB_ENDPOINT
# - YOUR_DB_PASSWORD
# - YOUR_ROLE_ARN

cat > task-definition.json << 'EOF'
{
  "family": "gotest1-task",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "256",
  "memory": "512",
  "executionRoleArn": "arn:aws:iam::YOUR_ACCOUNT_ID:role/ecsTaskExecutionRole",
  "containerDefinitions": [
    {
      "name": "gotest1-app",
      "image": "YOUR_ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/gotest1:latest",
      "essential": true,
      "portMappings": [
        {
          "containerPort": 8080,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {
          "name": "DATABASE_URL",
          "value": "postgres://gotest1:YOUR_DB_PASSWORD@YOUR_DB_ENDPOINT:5432/gotest1?sslmode=require"
        },
        {
          "name": "PORT",
          "value": "8080"
        },
        {
          "name": "APP_ENV",
          "value": "production"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/gotest1",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "ecs"
        }
      }
    }
  ]
}
EOF
```

### Create CloudWatch Log Group

```bash
aws logs create-log-group --log-group-name /ecs/gotest1
```

### Register Task Definition

```bash
aws ecs register-task-definition \
  --cli-input-json file://task-definition.json
```

---

## Step 7: Create Security Groups

### Get your VPC ID

```bash
export VPC_ID=$(aws ec2 describe-vpcs --filters "Name=isDefault,Values=true" --query 'Vpcs[0].VpcId' --output text)
echo "VPC ID: $VPC_ID"
```

### Create Security Group for ECS Tasks

```bash
# Create security group
aws ec2 create-security-group \
  --group-name gotest1-ecs-sg \
  --description "Security group for gotest1 ECS tasks" \
  --vpc-id $VPC_ID

# Get security group ID
export ECS_SG_ID=$(aws ec2 describe-security-groups \
  --filters "Name=group-name,Values=gotest1-ecs-sg" \
  --query 'SecurityGroups[0].GroupId' \
  --output text)

echo "ECS Security Group ID: $ECS_SG_ID"

# Allow inbound traffic on port 8080
aws ec2 authorize-security-group-ingress \
  --group-id $ECS_SG_ID \
  --protocol tcp \
  --port 8080 \
  --cidr 0.0.0.0/0
```

### Update RDS Security Group

```bash
# Get RDS security group ID
export RDS_SG_ID=$(aws rds describe-db-instances \
  --db-instance-identifier gotest1-db \
  --query 'DBInstances[0].VpcSecurityGroups[0].VpcSecurityGroupId' \
  --output text)

echo "RDS Security Group ID: $RDS_SG_ID"

# Allow ECS tasks to connect to RDS
aws ec2 authorize-security-group-ingress \
  --group-id $RDS_SG_ID \
  --protocol tcp \
  --port 5432 \
  --source-group $ECS_SG_ID
```

---

## Step 8: Create and Run ECS Service

### Get Subnet IDs

```bash
aws ec2 describe-subnets \
  --filters "Name=vpc-id,Values=$VPC_ID" \
  --query 'Subnets[*].SubnetId' \
  --output text
```

### Create ECS Service

```bash
# Replace SUBNET_ID_1 and SUBNET_ID_2 with actual subnet IDs from above
aws ecs create-service \
  --cluster gotest1-cluster \
  --service-name gotest1-service \
  --task-definition gotest1-task \
  --desired-count 1 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[SUBNET_ID_1,SUBNET_ID_2],securityGroups=[$ECS_SG_ID],assignPublicIp=ENABLED}"
```

---

## Step 9: Get Your Application URL

```bash
# Wait a minute for the task to start, then get the public IP
aws ecs list-tasks \
  --cluster gotest1-cluster \
  --service-name gotest1-service \
  --query 'taskArns[0]' \
  --output text

# Get task details (replace TASK_ARN with output from above)
aws ecs describe-tasks \
  --cluster gotest1-cluster \
  --tasks TASK_ARN \
  --query 'tasks[0].attachments[0].details[?name==`networkInterfaceId`].value' \
  --output text

# Get public IP (replace NETWORK_INTERFACE_ID)
aws ec2 describe-network-interfaces \
  --network-interface-ids NETWORK_INTERFACE_ID \
  --query 'NetworkInterfaces[0].Association.PublicIp' \
  --output text
```

### Test Your API

```bash
# Replace with your public IP
curl http://YOUR_PUBLIC_IP:8080/api/v1/users
```

---

## Step 10: Optional - Add Application Load Balancer

For production, you should use an Application Load Balancer with a custom domain.

This is more advanced and I can help you set it up once basic deployment is working.

---

## Monitoring and Debugging

### View Logs

```bash
# Get log stream name
aws logs describe-log-streams \
  --log-group-name /ecs/gotest1 \
  --order-by LastEventTime \
  --descending \
  --max-items 1 \
  --query 'logStreams[0].logStreamName' \
  --output text

# View logs (replace LOG_STREAM_NAME)
aws logs get-log-events \
  --log-group-name /ecs/gotest1 \
  --log-stream-name LOG_STREAM_NAME \
  --limit 50
```

### Check Service Status

```bash
aws ecs describe-services \
  --cluster gotest1-cluster \
  --services gotest1-service
```

---

## Cost Estimation (Monthly)

- **RDS db.t3.micro**: ~$15-20/month (750 hours free tier for 12 months)
- **ECS Fargate**: ~$5-10/month for 1 task
- **ECR**: First 500 MB free
- **Data Transfer**: First 100 GB free

**Total**: ~$20-30/month (or FREE for first 12 months with free tier)

---

## Cleanup (To Avoid Charges)

When you want to delete everything:

```bash
# Delete ECS service
aws ecs delete-service \
  --cluster gotest1-cluster \
  --service gotest1-service \
  --force

# Delete ECS cluster
aws ecs delete-cluster --cluster gotest1-cluster

# Delete RDS instance
aws rds delete-db-instance \
  --db-instance-identifier gotest1-db \
  --skip-final-snapshot

# Delete ECR repository
aws ecr delete-repository \
  --repository-name gotest1 \
  --force

# Delete security groups
aws ec2 delete-security-group --group-id $ECS_SG_ID
```

---

## Troubleshooting

### Task fails to start

1. Check CloudWatch logs for errors
2. Verify DATABASE_URL is correct
3. Ensure security groups allow traffic

### Can't connect to database

1. Check RDS security group allows connections from ECS security group
2. Verify DATABASE_URL has correct endpoint
3. Ensure RDS is in "available" state

### Image pull errors

1. Verify ECR repository exists
2. Check task execution role has ECR permissions
3. Re-run ECR login and push image

---

## Next Steps

1. Set up custom domain with Route 53
2. Add Application Load Balancer
3. Enable HTTPS with ACM (AWS Certificate Manager)
4. Set up CloudWatch alarms
5. Configure auto-scaling
6. Implement CI/CD with GitHub Actions

Need help with any of these steps? Let me know!
