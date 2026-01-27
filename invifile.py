import http.server
import socketserver
import os
import io
import urllib.parse
import html
import sys
from PIL import Image

class MyFileServer(http.server.SimpleHTTPRequestHandler):

    def generate_thumbnail(self, path):
        """Helper to create a small JPEG version of an image."""
        try:
            with Image.open(path) as img:
                img.thumbnail((150, 150))
                tmp = io.BytesIO()
                img.save(tmp, format="JPEG", quality=80)
                tmp.seek(0)
                return tmp
        except Exception as e:
            print(f"Error generating thumbnail: {e}")
            return None

    def do_GET(self):
        """Intercepts thumbnail requests or handles normal files."""
        if self.path.startswith("/_thumb/"):
            parts = self.path.split("/_thumb/", 1)
            requested_path = urllib.parse.unquote(parts[1])
            fs_path = self.translate_path(requested_path)

            thumb_data = self.generate_thumbnail(fs_path)
            if thumb_data:
                self.send_response(200)
                self.send_header("Content-type", "image/jpeg")
                self.end_headers()
                self.wfile.write(thumb_data.read())
                return
            else:
                self.send_error(404, "Thumbnail not available")
                return
        return super().do_GET()

    def do_POST(self):
            """Improved file upload handler."""
            try:
                content_type = self.headers['Content-Type']
                if not content_type or 'multipart/form-data' not in content_type:
                    self.send_error(400, "Bad Request")
                    return
                content_length = int(self.headers['Content-Length'])
                boundary = content_type.split("boundary=")[1].encode()
                body = self.rfile.read(content_length)
                try:
                    fn_start = body.find(b'filename="') + 10
                    fn_end = body.find(b'"', fn_start)
                    filename = body[fn_start:fn_end].decode('utf-8')

                    if not filename:
                        self.send_error(400, "No filename found")
                        return
                    head_end = body.find(b'\r\n\r\n', fn_start) + 4
                    data_end = body.find(b'--' + boundary, head_end) - 2
                    file_content = body[head_end:data_end]
                    current_dir = self.translate_path(self.path)
                    if os.path.isdir(current_dir):
                        target_path = os.path.join(current_dir, filename)
                    else:
                        target_path = os.path.join(os.path.dirname(current_dir), filename)

                    with open(target_path, 'wb') as f:
                        f.write(file_content)
                    self.send_response(303)
                    self.send_header('Location', self.path)
                    self.end_headers()

                except Exception as e:
                    print(f"Internal Parse Error: {e}")
                    self.send_error(500, f"Upload failed: {e}")

            except Exception as e:
                self.send_error(500, f"Server error: {e}")

    def list_directory(self, path):
        """Modified version of the original to add CSS grid and icons."""
        try:
            file_list = os.listdir(path)
        except OSError:
            self.send_error(404, "No permission to list directory")
            return None
        file_list.sort(key=lambda a: a.lower())
        r = []
        enc = sys.getfilesystemencoding()
        r.append('<!DOCTYPE HTML><html lang="en"><head>')
        r.append(f'<meta charset="{enc}"><meta name="viewport" content="width=device-width, initial-scale=1">')
        r.append('<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css">')
        r.append('''<style>
            body { font-family: sans-serif; background: #121212; color: #e0e0e0; padding: 20px; }
            .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 15px; }
            .item { text-align: center; background: #1e1e1e; padding: 10px; border-radius: 10px; transition: 0.2s; }
            .item:hover { background: #2c2c2c; }
            .item a { text-decoration: none; color: inherit; }
            .thumb-container { width: 120px; height: 120px; margin: 0 auto; display: flex; align-items: center; justify-content: center; }
            .item img { max-width: 100%; max-height: 100%; border-radius: 5px; }
            .item i { font-size: 60px; color: #555; }
            .label { display: block; margin-top: 8px; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        </style>''')
        r.append('<title>InviFiles</title></head>')
        r.append('<body>')
        r.append('''
                <div style="max-width: 1000px; margin: 0 auto;">
                    <h1 style="color: #6200ee; text-align: center; margin-bottom: 10px;">InviFiles</h1>

                    <div style="background: #1e1e1e; padding: 20px; border-radius: 12px; margin-bottom: 30px; border: 1px solid #333;">
                        <form enctype="multipart/form-data" method="post" style="display: flex; justify-content: center; align-items: center; gap: 10px; flex-wrap: wrap;">
                            <input name="file" type="file" style="color: #ccc; border: 1px solid #444; padding: 5px; border-radius: 5px;" />
                            <button type="submit" style="background: #6200ee; color: white; border: none; padding: 10px 20px; border-radius: 5px; cursor: pointer; font-weight: bold;">
                                <i class="fa-solid fa-upload"></i> Upload
                            </button>
                        </form>
                    </div>
                ''')
        r.append('<div class="grid">')
        r.append('</div>')
        r.append('</div>')
        r.append('</body></html>')
        for name in file_list:
            fullname = os.path.join(path, name)
            linkname = name + ("/" if os.path.isdir(fullname) else "")
            encoded_name = urllib.parse.quote(linkname)
            if os.path.isdir(fullname):
                icon_html = '<i class="fa-solid fa-folder" style="color: #ffd700;"></i>'
            elif name.lower().endswith(('.png', '.jpg', '.jpeg', '.webp')):
                icon_html = f'<img src="/_thumb/{encoded_name}" loading="lazy">'
            elif name.lower().endswith(('.mp4', '.mkv', '.mov')):
                icon_html = '<i class="fa-solid fa-file-video" style="color: #ff4444;"></i>'
            else:
                icon_html = '<i class="fa-solid fa-file"></i>'
            r.append(f'''
                <div class="item">
                    <a href="{encoded_name}">
                        <div class="thumb-container">{icon_html}</div>
                        <span class="label">{html.escape(name)}</span>
                    </a>
                </div>''')
        r.append('</div></body></html>')
        encoded = '\n'.join(r).encode(enc, 'surrogateescape')
        f = io.BytesIO()
        f.write(encoded)
        f.seek(0)
        self.send_response(200)
        self.send_header("Content-type", "text/html; charset=%s" % enc)
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        return f

if __name__ == "__main__":
    PORT = 9000
    socketserver.ThreadingTCPServer.allow_reuse_address = True
    try:
        with socketserver.ThreadingTCPServer(("", PORT), MyFileServer) as httpd:
            print(f":) InviFiles is running at http://localhost:{PORT}")
            print(" [Ctrl+C] to stop the server and release the port immediately.")
            httpd.serve_forever()
    except KeyboardInterrupt:
        print("\n :( Shutting down InviFiles...")
        sys.exit(0)
