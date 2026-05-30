FROM node:20-alpine
WORKDIR /app
COPY package.json ./
RUN npm install --production
COPY server.js routes.json ./
EXPOSE 3001
CMD ["node", "server.js"]