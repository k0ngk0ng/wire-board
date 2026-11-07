"""Test-only byte-preserving TCP delays in front of the real platform fixture.

No application messages or game states are synthesized. Browser HTTP/WebSocket
connections still terminate at the real Go host. Gates only hold returned bytes.
"""
import socket, socketserver, threading
from urllib.parse import urlparse

class BrowserNetwork:
 def __init__(self,origin):
  upstream=urlparse(origin);assert upstream.scheme=='http'and upstream.hostname in ('127.0.0.1','localhost')
  self.stream=threading.Event();self.stream.set();self.checkpoint=threading.Event();self.checkpoint.set();self.waiting=threading.Event();owner=self
  class Handler(socketserver.BaseRequestHandler):
   def handle(self):
    remote=socket.create_connection((upstream.hostname,upstream.port));kind={'stream':False,'checkpoint':False}
    def upload():
     try:
      while data:=self.request.recv(65536):
       if b'upgrade: websocket'in data.lower():kind['stream']=True
       if b'GET /api/rooms/'in data and b'/sync'in data:kind['checkpoint']=True
       remote.sendall(data)
     except OSError:pass
     finally:
      try:remote.shutdown(socket.SHUT_WR)
      except OSError:pass
    threading.Thread(target=upload,daemon=True).start()
    try:
     while data:=remote.recv(65536):
      if kind['stream']:owner.stream.wait(5)
      if kind['checkpoint']:
       if not owner.checkpoint.is_set():owner.waiting.set()
       owner.checkpoint.wait(5);kind['checkpoint']=False
      self.request.sendall(data)
    except OSError:pass
    finally:remote.close()
  class Server(socketserver.ThreadingTCPServer):allow_reuse_address=True;daemon_threads=True
  self.server=Server(('127.0.0.1',0),Handler);self.url='http://127.0.0.1:'+str(self.server.server_address[1]);threading.Thread(target=self.server.serve_forever,daemon=True).start()
 def close(self):
  self.stream.set();self.checkpoint.set();self.server.shutdown();self.server.server_close()
