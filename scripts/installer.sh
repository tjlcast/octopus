#!/bin/bash
set -e

# ========== 默认配置（可通过环境变量覆盖） ==========
: "${HOST_PORT:=5000}"               # 主机映射端口，默认 5000
: "${CONTAINER_PORT:=5000}"          # 容器内部端口（请根据镜像实际监听端口修改）
: "${DATA_DIR:=$HOME/.octopus-deploy/data}"  # 数据目录，默认固定在用户家目录下，不依赖调用位置
: "${CONTAINER_NAME:=octopus-app}"   # 容器名称，固定

# ========== 颜色输出 ==========
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# ========== 帮助函数 ==========
show_help() {
    cat << EOF
Octopus Deploy 安装程序

用法:
    $0 [选项]

选项:
    -h, --help     显示此帮助信息并退出

环境变量（用于覆盖默认配置）:
    HOST_PORT       主机映射端口（默认: 5000）
    CONTAINER_PORT  容器内部端口（默认: 5000，请根据镜像实际端口修改）
    DATA_DIR        数据目录，强烈建议传绝对路径（默认: \$HOME/.octopus-deploy/data）
    CONTAINER_NAME  容器名称（默认: octopus-app）

使用示例:
    # 使用默认配置运行（数据落在 ~/.octopus-deploy/data）
    ./octopus-installer.run

    # 修改主机端口为 8080
    HOST_PORT=8080 ./octopus-installer.run

    # 指定数据目录为当前调用目录下的 mydata（注意用 \$(pwd) 显式求值绝对路径）
    DATA_DIR="\$(pwd)/mydata" ./octopus-installer.run

    # 同时修改多个参数
    HOST_PORT=9090 DATA_DIR="\$(pwd)/mydata" CONTAINER_NAME=myapp ./octopus-installer.run

    # 保留临时解压目录（调试用，makeself 自带参数）
    ./octopus-installer.run --keep

    # sample
    HOST_PORT=8081 DATA_DIR="\$(pwd)/octopus-data" ./octopus-installer.run

默认行为:
    1. 加载 Docker 镜像（随安装包一起打包的 Docker-jialtang-octopus-*.tar）
    2. 创建数据目录（若不存在）
    3. 删除同名的旧容器（如果存在）
    4. 启动新容器，挂载数据卷，映射端口
    5. 容器以 --restart unless-stopped 策略运行

重要提示（关于 DATA_DIR 路径）:
    本安装程序由 makeself 打包，运行时会先解压到 /tmp 下的临时目录并 cd 进去，
    因此脚本内部无法可靠获知你执行 .run 文件时所在的原始目录。

    - 若不指定 DATA_DIR，数据将固定存放在 \$HOME/.octopus-deploy/data，与调用位置无关，安全可靠。
    - 若你想让数据落在当前目录下，请显式传入绝对路径，例如：
          DATA_DIR="\$(pwd)/octopus-data" ./octopus-installer.run
      \$(pwd) 会在你调用时所在的 shell 中被求值，因此结果准确。
    - 若传入相对路径（如 DATA_DIR=mydata），脚本会尝试猜测原始目录，
      但该猜测在部分环境/makeself 版本下可能不准确，请勿在生产环境依赖此行为。
EOF
    exit 0
}

# ========== 参数解析 ==========
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            show_help
            ;;
        *)
            # 忽略其他未知参数（makeself 可能传 --keep 等）
            shift
            ;;
    esac
done

# makeself 解压后，脚本与镜像 tar 位于同一目录，切到该目录以保证相对路径可靠
cd "$(dirname "$0")"

# ========== 主逻辑开始 ==========
echo -e "${GREEN}>>> Octopus Deploy 安装程序启动${NC}"

# ========== 解析数据目录的绝对路径 ==========
if [[ "${DATA_DIR}" == /* ]]; then
    # 情况一：DATA_DIR 本身就是绝对路径（包括默认值 $HOME/... 以及用户显式传入的 $(pwd)/xxx）
    DATA_ABS_PATH="${DATA_DIR}"
    echo "数据目录（绝对路径）: ${DATA_ABS_PATH}"
else
    # 情况二：DATA_DIR 是相对路径，尝试猜测原始执行目录作为兜底
    echo -e "${YELLOW}警告: DATA_DIR='${DATA_DIR}' 是相对路径，脚本将尝试猜测原始执行目录。${NC}"
    echo -e "${YELLOW}      该猜测可能不准确，建议改用绝对路径，例如: DATA_DIR=\"\$(pwd)/${DATA_DIR}\"${NC}"

    if [ -n "$USER_PWD" ]; then
        ORIGINAL_PWD="$USER_PWD"
        echo "检测到原始执行目录（通过 USER_PWD）: ${ORIGINAL_PWD}"
    elif [ -n "$OLDPWD" ] && [ "$OLDPWD" != "$PWD" ]; then
        ORIGINAL_PWD="$OLDPWD"
        echo "检测到原始执行目录（通过 OLDPWD）: ${ORIGINAL_PWD}"
    else
        ORIGINAL_PWD="$(pwd)"
        echo -e "${RED}警告: 无法获取原始执行目录，将退化使用当前目录（可能是 /tmp 下的临时目录！）: ${ORIGINAL_PWD}${NC}"
    fi

    DATA_ABS_PATH="${ORIGINAL_PWD}/${DATA_DIR}"
    echo "数据目录（拼接后）: ${DATA_ABS_PATH}"
fi

# 检查 Docker 可用性
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: 未找到 docker 命令，请确保 Docker 已安装并启动。${NC}"
    exit 1
fi

# 加载 Docker 镜像（随安装包一起打包）
image_tar="$(
    find . -maxdepth 1 -type f -name 'Docker-jialtang-octopus-*.tar' \
        -print -quit
)"
if [[ -z "${image_tar}" ]]; then
    echo -e "${RED}错误: 未找到 Docker 镜像文件（Docker-jialtang-octopus-*.tar）。${NC}"
    exit 1
fi

echo "正在加载 Docker 镜像: ${image_tar}"
load_output="$(docker load -i "${image_tar}")"
echo "${load_output}"

image_name="$(
    echo "${load_output}" \
        | awk -F': ' '/Loaded image:/ { image=$2 } END { print image }'
)"
if [[ -z "${image_name}" ]]; then
    echo -e "${RED}错误: 无法从 docker load 输出中解析镜像名称。${NC}"
    exit 1
fi

# 创建数据目录
mkdir -p "${DATA_ABS_PATH}"
echo "数据目录已就绪: ${DATA_ABS_PATH}"

# 处理旧容器（如果存在则停止并删除）
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo -e "${YELLOW}发现已存在的容器 ${CONTAINER_NAME}，正在停止并删除...${NC}"
    docker stop "${CONTAINER_NAME}" 2>/dev/null || true
    docker rm "${CONTAINER_NAME}" 2>/dev/null || true
fi

# 启动新容器
echo "正在启动容器: ${CONTAINER_NAME}"
docker run -d \
    --name "${CONTAINER_NAME}" \
    --restart unless-stopped \
    -v "${DATA_ABS_PATH}:/app/data" \
    -p "${HOST_PORT}:${CONTAINER_PORT}" \
    "${image_name}"

# 输出结果
echo -e "${GREEN}>>> 容器已成功启动！${NC}"
echo "  容器名称: ${CONTAINER_NAME}"
echo "  主机端口: ${HOST_PORT} -> 容器端口: ${CONTAINER_PORT}"
echo "  数据目录: ${DATA_ABS_PATH}"
echo ""
echo "查看日志: docker logs ${CONTAINER_NAME}"
echo "停止容器: docker stop ${CONTAINER_NAME}"
