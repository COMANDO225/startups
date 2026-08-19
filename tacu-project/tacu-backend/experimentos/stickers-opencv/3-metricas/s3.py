import cv2, numpy as np

G = "/home/anderson/Projects/startups/tacu-project/tacu-backend/testdata/cartas/galponcito.jpeg"
P2 = "/home/anderson/Projects/startups/tacu-project/tacu-backend/testdata/cartas/prueba.jpg"
img = cv2.imread(G)

def run(im, smin=90, vmin=120):
    hsv = cv2.cvtColor(im, cv2.COLOR_BGR2HSV)
    m = cv2.inRange(hsv, (20, smin, vmin), (40, 255, 255))
    k = cv2.getStructuringElement(cv2.MORPH_RECT, (7, 7))
    m = cv2.morphologyEx(m, cv2.MORPH_CLOSE, k); m = cv2.morphologyEx(m, cv2.MORPH_OPEN, k)
    n, lab, st, _ = cv2.connectedComponentsWithStats(m, 8)
    res = []
    for i in range(1, n):
        x, y, w, h, a = st[i]
        if a < 900: continue
        f, ar = a/float(w*h), w/float(h)
        if f < 0.75: continue
        if a > 12000:
            if f >= 0.80 and ar < 0.8 and a < 40000: res.append(('col', x, y, w, h, a))
            continue
        if a >= 4500 and 0.9 <= ar <= 2.2: res.append(('stk', x, y, w, h, a))
    return res

# 1) el panel amarillo COMBO FAMILIAR: que tono es vs el sticker
hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)
def stats(x, y, w, h, name):
    r = hsv[y:y+h, x:x+w]
    print(f"  {name:18s} H={r[:,:,0].mean():.0f} S={r[:,:,1].mean():.0f} V={r[:,:,2].mean():.0f}")
print("tono medio de regiones:")
stats(830, 315, 60, 40, "sticker 49.0")
stats(1440, 700, 60, 40, "panel COMBO FAM")
stats(1210, 690, 60, 40, "sticker 55.0")
stats(100, 1115, 40, 20, "digitos telefono")

# 2) sensibilidad a iluminacion: gamma + brillo
print("\nsensibilidad a iluminacion (n detecciones, base=12):")
for g in (0.6, 0.8, 1.0, 1.3, 1.7):
    lut = np.array([((i/255.0)**g)*255 for i in range(256)]).astype("uint8")
    print(f"  gamma {g}: {len(run(cv2.LUT(img, lut)))}")
for d in (-60, -30, 30, 60):
    print(f"  brillo {d:+d}: {len(run(cv2.convertScaleAbs(img, alpha=1, beta=d)))}")
for q in (30, 50, 70):
    ok, enc = cv2.imencode('.jpg', img, [cv2.IMWRITE_JPEG_QUALITY, q])
    print(f"  jpeg q{q}: {len(run(cv2.imdecode(enc, 1)))}")
# rotacion (foto torcida)
for ang in (-8, -4, 4, 8):
    M = cv2.getRotationMatrix2D((800, 600), ang, 1.0)
    print(f"  rot {ang:+d}deg: {len(run(cv2.warpAffine(img, M, (1600, 1200))))}")

# 3) otra carta sin stickers -> falsos positivos?
p2 = cv2.imread(P2)
r2 = run(p2)
print(f"\nprueba.jpg {p2.shape[1]}x{p2.shape[0]}: {len(r2)} detecciones {r2}")
