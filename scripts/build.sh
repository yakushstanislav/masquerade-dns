set -e

check_variable()
{
    local name="$1"
    local value=$2

    if [ -z $value ]; then
        echo "Variable \"$name\" is not set"
        exit 1
    fi
}

check_variable "Application name" $APP_NAME
check_variable "Application version" $APP_VERSION
check_variable "Application directory" $APP_DIR
check_variable "Build directory" $BUILD_DIR
check_variable "Build OS" $OS
check_variable "Build architecture" $ARCH

cd $APP_DIR
GOOS=$OS GOARCH=$ARCH go build -o $BUILD_DIR/${APP_VERSION}/${APP_NAME}_${OS}_${ARCH}