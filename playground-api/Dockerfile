# Start from a lightweight Rust base image
FROM rust:1.76-slim

# Install curl, then use it to install Node.js (v20)
RUN apt-get update && apt-get install -y curl && \
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Add the WebAssembly target required for compiling Soroban contracts
RUN rustup target add wasm32-unknown-unknown

# Set up the application directory
WORKDIR /app

# Install Node dependencies.
#
# npm ci, not npm install: `install` is free to resolve a caret range to
# whatever was published most recently, so a compromised patch release of a
# transitive package lands in the image without the lockfile ever changing.
# `ci` installs exactly what the lockfile pins.
#
# --ignore-scripts because an install hook is the first code a compromised
# package gets to run, and it runs here as root, during a build, on a machine
# that can reach the network. Nothing in this dependency tree needs one.
COPY package*.json ./
RUN npm ci --omit=dev --ignore-scripts

# Copy the server code
COPY server.js ./

# Render assigns a PORT dynamically, but we expose 8080 as a fallback
ENV PORT=8080
EXPOSE 8080

# Start the Express API
CMD ["node", "server.js"]
