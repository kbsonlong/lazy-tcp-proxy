# docker build . -tag test-npm-socket && docker run -it --rm -p 3000:3000 test-npm-socket
FROM node:25-alpine
WORKDIR /app
COPY index.js /app/index.js
EXPOSE 3000
CMD ["node", "index.js"]