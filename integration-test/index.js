const net = require('net');

const server = net.createServer((socket) => {
  socket.on('data', () => {
    socket.write(
      'HTTP/1.1 200 OK\r\n' +
      'Content-Type: text/html\r\n' +
      'Content-Length: 20\r\n' +
      'Connection: close\r\n' +
      '\r\n' +
      '<h1>hello world</h1>'
    );

    socket.end(); // close connection
  });
});

server.listen(3000, () => {
  console.log('Server running at http://localhost:3000/');
});

// on SIGINT, gracefully shutdown the server
process.on('SIGINT', () => {
  console.log('Shutting down server...');
  server.close(() => {
    console.log('Server shut down gracefully.');
    process.exit(0);
  });
});