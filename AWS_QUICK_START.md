# AWS Deployment - Quick Start

This is a simplified guide to get you started. For detailed explanations, see [AWS_DEPLOYMENT_GUIDE.md](./AWS_DEPLOYMENT_GUIDE.md).

## Prerequisites (One-time setup)

### 1. Create AWS Account
- Go to [aws.amazon.com](https://aws.amazon.com) and create an account
- Requires credit card (but we'll stay in free tier)

### 2. Install AWS CLI
```bash
# macOS
curl "https://awscli.amazonaws.com/AWSCLIV2.pkg" -o "AWSCLIV2.pkg"
sudo installer -pkg AWSCLIV2.pkg -target /
```

### 3. Configure AWS CLI
1. In AWS Console, go to: Your Name (top right) → Security credentials
2. Create access key → Choose "CLI"
3. Download and save the keys

```bash
aws configure
# Enter your Access Key ID
# Enter your Secret Access Key
# Region: us-east-1
# Output: json
```

## Deployment Steps

### Step 1: Create Database (One-time)

**Option 1: AWS Console (Easier)**
1. Go to [RDS Console](https://console.aws.amazon.com/rds)
2. Click "Create database"
3. Settings:
   - Engine: **PostgreSQL**
   - Template: **Free tier**
   - DB identifier: **gotest1-db**
   - Username: **gotest1**
   - Password: **[create strong password]**
   - Public access: **Yes**
   - Initial DB name: **gotest1**
4. Click "Create database"
5. Wait 10 minutes
6. Copy the **Endpoint** (save it!)

**Option 2: AWS CLI**
```bash
aws rds create-db-instance \
  --db-instance-identifier gotest1-db \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --master-username gotest1 \
  --master-user-password YOUR_PASSWORD_HERE \
  --allocated-storage 20 \
  --db-name gotest1 \
  --publicly-accessible
```

### Step 2: Run Migration

```bash
# Wait for RDS to be "available", then:
export DB_ENDPOINT="your-rds-endpoint.amazonaws.com"
export DB_PASSWORD="your-password"

PGPASSWORD=$DB_PASSWORD psql \
  -h $DB_ENDPOINT \
  -U gotest1 \
  -d gotest1 \
  -f db/migrations/001_create_users.sql
```

### Step 3: Deploy Application

```bash
# Use the helper script
./deploy-to-aws.sh

# Follow the menu:
# 1. Create ECR repository
# 2. Build and push Docker image
# 3. Create ECS cluster
# 4. Create task execution role
```

### Step 4: Create Task Definition

You need to manually create the task definition file with your details:

```bash
# Get your AWS account ID
aws sts get-caller-identity --query Account --output text

# Create task definition
cat > task-definition.json << EOF
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
          "value": "postgres://gotest1:YOUR_PASSWORD@YOUR_RDS_ENDPOINT:5432/gotest1?sslmode=require"
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
          "awslogs-stream-prefix": "ecs",
          "awslogs-create-group": "true"
        }
      }
    }
  ]
}
EOF
```

**Replace:**
- `YOUR_ACCOUNT_ID` (get with: `aws sts get-caller-identity --query Account --output text`)
- `YOUR_PASSWORD` (your RDS password)
- `YOUR_RDS_ENDPOINT` (your RDS endpoint from Step 1)

### Step 5: Register Task and Create Service

```bash
# Create log group
aws logs create-log-group --log-group-name /ecs/gotest1

# Register task definition
aws ecs register-task-definition --cli-input-json file://task-definition.json

# Get VPC and Security Group info
export VPC_ID=$(aws ec2 describe-vpcs --filters "Name=isDefault,Values=true" --query 'Vpcs[0].VpcId' --output text)

# Create security group for ECS
aws ec2 create-security-group \
  --group-name gotest1-ecs-sg \
  --description "Security group for gotest1" \
  --vpc-id $VPC_ID

export ECS_SG_ID=$(aws ec2 describe-security-groups \
  --filters "Name=group-name,Values=gotest1-ecs-sg" \
  --query 'SecurityGroups[0].GroupId' \
  --output text)

# Allow traffic on port 8080
aws ec2 authorize-security-group-ingress \
  --group-id $ECS_SG_ID \
  --protocol tcp \
  --port 8080 \
  --cidr 0.0.0.0/0

# Update RDS security group to allow ECS
export RDS_SG_ID=$(aws rds describe-db-instances \
  --db-instance-identifier gotest1-db \
  --query 'DBInstances[0].VpcSecurityGroups[0].VpcSecurityGroupId' \
  --output text)

aws ec2 authorize-security-group-ingress \
  --group-id $RDS_SG_ID \
  --protocol tcp \
  --port 5432 \
  --source-group $ECS_SG_ID

# Get subnet IDs
export SUBNET_1=$(aws ec2 describe-subnets --filters "Name=vpc-id,Values=$VPC_ID" --query 'Subnets[0].SubnetId' --output text)
export SUBNET_2=$(aws ec2 describe-subnets --filters "Name=vpc-id,Values=$VPC_ID" --query 'Subnets[1].SubnetId' --output text)

# Create ECS service
aws ecs create-service \
  --cluster gotest1-cluster \
  --service-name gotest1-service \
  --task-definition gotest1-task \
  --desired-count 1 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[$SUBNET_1,$SUBNET_2],securityGroups=[$ECS_SG_ID],assignPublicIp=ENABLED}"
```

### Step 6: Get Your Application URL

```bash
# Use the helper script
./deploy-to-aws.sh
# Choose option 7: Get application URL

# Or manually:
TASK_ARN=$(aws ecs list-tasks --cluster gotest1-cluster --service-name gotest1-service --query 'taskArns[0]' --output text)

NETWORK_INTERFACE_ID=$(aws ecs describe-tasks --cluster gotest1-cluster --tasks $TASK_ARN --query 'tasks[0].attachments[0].details[?name==`networkInterfaceId`].value' --output text)

PUBLIC_IP=$(aws ec2 describe-network-interfaces --network-interface-ids $NETWORK_INTERFACE_ID --query 'NetworkInterfaces[0].Association.PublicIp' --output text)

echo "Your API: http://$PUBLIC_IP:8080/api/v1/users"
```

### Step 7: Test!

```bash
curl http://YOUR_PUBLIC_IP:8080/api/v1/users

curl -X POST http://YOUR_PUBLIC_IP:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"AWS User","email":"aws@example.com"}'
```

## Troubleshooting

### Check logs
```bash
./deploy-to-aws.sh
# Choose option 8: View logs
```

### Check service status
```bash
aws ecs describe-services --cluster gotest1-cluster --services gotest1-service
```

### Task not running?
1. Check CloudWatch logs
2. Verify DATABASE_URL is correct in task definition
3. Ensure security groups allow traffic

## Update Your Application

When you make code changes:

```bash
# 1. Build and push new image
./deploy-to-aws.sh
# Choose option 2

# 2. Force new deployment
aws ecs update-service \
  --cluster gotest1-cluster \
  --service gotest1-service \
  --force-new-deployment
```

## Monthly Cost

- RDS db.t3.micro: ~$15-20 (FREE first 12 months)
- ECS Fargate: ~$5-10
- Total: ~$20-30/month (or FREE first year)

## Cleanup

To delete everything and stop charges:

```bash
# Delete service
aws ecs delete-service --cluster gotest1-cluster --service gotest1-service --force

# Delete cluster
aws ecs delete-cluster --cluster gotest1-cluster

# Delete RDS (CAREFUL - deletes all data!)
aws rds delete-db-instance --db-instance-identifier gotest1-db --skip-final-snapshot

# Delete ECR
aws ecr delete-repository --repository-name gotest1 --force
```

## Need Help?

1. Check [AWS_DEPLOYMENT_GUIDE.md](./AWS_DEPLOYMENT_GUIDE.md) for detailed explanations
2. Check CloudWatch logs for errors
3. Verify all security groups are configured correctly

## Next Steps

Once basic deployment is working:
1. Add custom domain (Route 53)
2. Add HTTPS (ACM + ALB)
3. Set up CI/CD (GitHub Actions)
4. Configure auto-scaling
5. Add CloudWatch alarms
