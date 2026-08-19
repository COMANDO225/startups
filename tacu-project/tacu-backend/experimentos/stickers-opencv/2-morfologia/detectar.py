import cv2, numpy as np, time

P = "/home/anderson/Projects/startups/tacu-project/tacu-backend/testdata/cartas/galponcito.jpeg"
img = cv2.imread(P)
H, W = img.shape[:2]

def blobs(img, smin, vmin, kk=7, amin=900):
    hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)
    m = cv2.inRange(hsv, (20, smin, vmin), (40, 255, 255))
    k = cv2.getStructuringElement(cv2.MORPH_RECT, (kk, kk))
    m = cv2.morphologyEx(m, cv2.MORPH_CLOSE, k)
    m = cv2.morphologyEx(m, cv2.MORPH_OPEN, k)
    n, lab, st, ce = cv2.connectedComponentsWithStats(m, 8)
    out = []
    for i in range(1, n):
        x, y, w, h, a = st[i]
        if a < amin: continue
        out.append((x, y, w, h, a, a/float(w*h), w/float(h)))
    return m, out

def keep(b, scale=1.0):
    x, y, w, h, a, fill, ar = b
    amin, amax = 4500*scale*scale, 12000*scale*scale
    if fill < 0.75: return False
    if a > amax:                      # blob grande: solo si es columna apilada
        return fill >= 0.80 and ar < 0.8 and a < 40000*scale*scale
    return a >= amin and 0.9 <= ar <= 2.2

m, bs = blobs(img, 90, 120)
kept = [b for b in bs if keep(b)]
print(f"total blobs {len(bs)} -> filtrados {len(kept)}")
stickers = 0
for b in sorted(kept, key=lambda z: -z[4]):
    tag = "COLUMNA-APILADA" if b[6] < 0.8 else "sticker"
    print(f"  {tag} x={b[0]} y={b[1]} {b[2]}x{b[3]} area={b[4]} fill={b[5]:.2f} ar={b[6]:.2f}")

# --- barrido de robustez de umbrales ---
print("\nrobustez (S_min, V_min) -> n_filtrados")
for s in (60, 80, 90, 110, 130):
    row = []
    for v in (90, 110, 120, 140, 160):
        _, bb = blobs(img, s, v)
        row.append(len([b for b in bb if keep(b)]))
    print(f"  S>={s}: " + " ".join(f"V{v}:{c}" for v, c in zip((90,110,120,140,160), row)))

# --- velocidad vs resolucion ---
print("\nvelocidad por resolucion (media de 30, sin decode)")
for scale in (1.0, 0.5, 0.25):
    im = cv2.resize(img, None, fx=scale, fy=scale) if scale != 1.0 else img
    hsv = cv2.cvtColor(im, cv2.COLOR_BGR2HSV)
    ts = []
    for _ in range(30):
        t = time.perf_counter()
        mm = cv2.inRange(hsv, (20, 90, 120), (40, 255, 255))
        k = cv2.getStructuringElement(cv2.MORPH_RECT, (7, 7))
        mm = cv2.morphologyEx(mm, cv2.MORPH_CLOSE, k)
        mm = cv2.morphologyEx(mm, cv2.MORPH_OPEN, k)
        cv2.connectedComponentsWithStats(mm, 8)
        ts.append((time.perf_counter()-t)*1000)
    _, bb = blobs(im, 90, 120, amin=int(900*scale*scale))
    kk = [b for b in bb if keep(b, scale)]
    print(f"  {im.shape[1]}x{im.shape[0]}: {np.mean(ts):.2f}ms  detecciones={len(kk)}")

# --- 1 hilo (VPS barato) ---
cv2.setNumThreads(1)
hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)
ts = []
for _ in range(20):
    t = time.perf_counter()
    mm = cv2.inRange(hsv, (20,90,120), (40,255,255))
    k = cv2.getStructuringElement(cv2.MORPH_RECT,(7,7))
    mm = cv2.morphologyEx(mm, cv2.MORPH_CLOSE, k); mm = cv2.morphologyEx(mm, cv2.MORPH_OPEN, k)
    cv2.connectedComponentsWithStats(mm, 8)
    ts.append((time.perf_counter()-t)*1000)
print(f"\n1 solo hilo, 1600x1200: {np.mean(ts):.2f}ms")

vis = img.copy()
for b in kept:
    cv2.rectangle(vis,(b[0],b[1]),(b[0]+b[2],b[1]+b[3]),(0,255,0),3)
cv2.imwrite("/tmp/claude-1000/-home-anderson-Projects-startups/396da9f5-16ea-4249-b510-feab9b1909aa/scratchpad/vis2.png", vis)
