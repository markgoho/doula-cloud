# Dockerfile for Doula Cloud CI builds
# Includes tools for building the Hugo marketing site
FROM ubuntu:22.04

# Avoid prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

# Set versions for our tools as arguments
ARG HUGO_VERSION=0.163.0
ARG BUN_VERSION=1.3.14

# 1. Install base dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
  curl \
  wget \
  ca-certificates \
  unzip \
  git \
  && rm -rf /var/lib/apt/lists/*

# Configure git to trust the workspace directory to avoid ownership errors in GitHub Actions
RUN git config --global --add safe.directory /__w/doula-cloud/doula-cloud && \
  git config --system --add safe.directory /__w/doula-cloud/doula-cloud && \
  git config --system --add core.quotepath false

# 2. Install Bun
RUN curl -fsSL https://bun.com/install | bash -s "bun-v${BUN_VERSION}"
ENV PATH="/root/.bun/bin:$PATH"

# 3. Install Hugo (Extended Version)
RUN wget "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-amd64.deb" \
  && apt-get install -y ./hugo_extended_${HUGO_VERSION}_linux-amd64.deb \
  && rm hugo_extended_${HUGO_VERSION}_linux-amd64.deb

# 4. Verify installations
RUN echo "Bun version: $(bun --version)"
RUN echo "Hugo version: $(hugo version)"

# Set the working directory for when the container starts
WORKDIR /workspace
