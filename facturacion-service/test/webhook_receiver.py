import http.server, json, sys, hmac, hashlib
SECRET=b'secreto-de-prueba'
class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        n=int(self.headers.get('Content-Length',0)); body=self.rfile.read(n)
        firma=self.headers.get('X-Facturacion-Signature','')
        esperada='sha256='+hmac.new(SECRET,body,hashlib.sha256).hexdigest()
        ok='FIRMA OK' if hmac.compare_digest(firma,esperada) else 'FIRMA INVALIDA'
        d=json.loads(body)
        print(f"[webhook] {ok} | {d['evento']} {d['numero']} -> {d['estado']} ({d.get('codigo_sunat')})",flush=True)
        self.send_response(200); self.end_headers(); self.wfile.write(b'{}')
    def log_message(self,*a): pass
http.server.HTTPServer(('0.0.0.0',9099),H).serve_forever()
