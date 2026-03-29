#!/bin/bash
set -e

echo "================================"
echo "AWS Deployment Helper Script"
echo "================================"
echo ""

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo "❌ AWS CLI is not installed. Please install it first:"
    echo "   curl \"https://awscli.amazonaws.com/AWSCLIV2.pkg\" -o \"AWSCLIV2.pkg\""
    echo "   sudo installer -pkg AWSCLIV2.pkg -target /"
    exit 1
fi

# Check if AWS is configured
if ! aws sts get-caller-identity &> /dev/null; then
    echo "❌ AWS CLI is not configured. Please run:"
    echo "   aws configure"
    exit 1
fi

echo "✅ AWS CLI is installed and configured"
echo ""

# Get AWS Account ID
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export AWS_REGION=${AWS_REGION:-us-east-1}

echo "📋 Configuration:"
echo "   AWS Account ID: $AWS_ACCOUNT_ID"
echo "   AWS Region: $AWS_REGION"
echo ""

# Menu
echo "What would you like to do?"
echo "1. Create ECR repository"
echo "2. Build and push Docker image"
echo "3. Create ECS cluster"
echo "4. Create task execution role"
echo "5. Register task definition"
echo "6. Create ECS service"
echo "7. Get application URL"
echo "8. View logs"
echo "9. Full deployment (all steps)"
echo "0. Exit"
echo ""
read -p "Enter your choice (0-9): " choice

case $choice in
    1)
        echo ""
        echo "Creating ECR repository..."
        aws ecr create-repository --repository-name gotest1 --region $AWS_REGION
        echo "✅ ECR repository created"
        ;;

    2)
        echo ""
        echo "Building and pushing Docker image..."

        # Login to ECR
        echo "Logging in to ECR..."
        aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com

        # Build image
        echo "Building Docker image..."
        docker build -t gotest1 .

        # Tag image
        echo "Tagging image..."
        docker tag gotest1:latest $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/gotest1:latest

        # Push image
        echo "Pushing image to ECR..."
        docker push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/gotest1:latest

        echo "✅ Docker image pushed successfully"
        echo "   Image URI: $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/gotest1:latest"
        ;;

    3)
        echo ""
        echo "Creating ECS cluster..."
        aws ecs create-cluster --cluster-name gotest1-cluster --region $AWS_REGION
        echo "✅ ECS cluster created"
        ;;

    4)
        echo ""
        echo "Creating task execution role..."

        # Create trust policy
        cat > /tmp/ecs-task-trust-policy.json << 'EOF'
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
          --assume-role-policy-document file:///tmp/ecs-task-trust-policy.json 2>/dev/null || echo "Role already exists"

        # Attach policy
        aws iam attach-role-policy \
          --role-name ecsTaskExecutionRole \
          --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy

        ROLE_ARN=$(aws iam get-role --role-name ecsTaskExecutionRole --query 'Role.Arn' --output text)

        echo "✅ Task execution role created"
        echo "   Role ARN: $ROLE_ARN"
        ;;

    7)
        echo ""
        echo "Getting application URL..."

        # Get task ARN
        TASK_ARN=$(aws ecs list-tasks \
          --cluster gotest1-cluster \
          --service-name gotest1-service \
          --region $AWS_REGION \
          --query 'taskArns[0]' \
          --output text)

        if [ "$TASK_ARN" == "None" ] || [ -z "$TASK_ARN" ]; then
            echo "❌ No running tasks found"
            exit 1
        fi

        echo "Task ARN: $TASK_ARN"

        # Get network interface ID
        NETWORK_INTERFACE_ID=$(aws ecs describe-tasks \
          --cluster gotest1-cluster \
          --tasks $TASK_ARN \
          --region $AWS_REGION \
          --query 'tasks[0].attachments[0].details[?name==`networkInterfaceId`].value' \
          --output text)

        echo "Network Interface ID: $NETWORK_INTERFACE_ID"

        # Get public IP
        PUBLIC_IP=$(aws ec2 describe-network-interfaces \
          --network-interface-ids $NETWORK_INTERFACE_ID \
          --region $AWS_REGION \
          --query 'NetworkInterfaces[0].Association.PublicIp' \
          --output text)

        echo ""
        echo "✅ Your application is running at:"
        echo "   http://$PUBLIC_IP:8080/api/v1/users"
        echo ""
        echo "Test with:"
        echo "   curl http://$PUBLIC_IP:8080/api/v1/users"
        ;;

    8)
        echo ""
        echo "Fetching recent logs..."

        # Get latest log stream
        LOG_STREAM=$(aws logs describe-log-streams \
          --log-group-name /ecs/gotest1 \
          --region $AWS_REGION \
          --order-by LastEventTime \
          --descending \
          --max-items 1 \
          --query 'logStreams[0].logStreamName' \
          --output text)

        if [ "$LOG_STREAM" == "None" ] || [ -z "$LOG_STREAM" ]; then
            echo "❌ No logs found"
            exit 1
        fi

        echo "Log stream: $LOG_STREAM"
        echo ""

        # Get logs
        aws logs get-log-events \
          --log-group-name /ecs/gotest1 \
          --log-stream-name $LOG_STREAM \
          --region $AWS_REGION \
          --limit 50 \
          --query 'events[*].message' \
          --output text
        ;;

    0)
        echo "Goodbye!"
        exit 0
        ;;

    *)
        echo "❌ Invalid choice"
        exit 1
        ;;
esac

echo ""
echo "✅ Done!"
