FROM node:18-alpine
RUN apk add --no-cache stress-ng iproute2 iptables
WORKDIR /app
COPY package.json .
RUN npm install
COPY index.js .
CMD ["node", "index.js"]
