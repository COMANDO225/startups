import cv2, numpy as np, time, sys, json

P = "/home/anderson/Projects/startups/tacu-project/tacu-backend/testdata/cartas/galponcito.jpeg"

def detect(path, verbose=True):
    t0 = time.perf_counter()
    img = cv2.imread(path)
    t_read = time.perf_counter() - t0

    t1 = time.perf_counter()
    hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)
    # amarillo saturado: H 20-40 (OpenCV 0-179), S alto, V alto
    mask = cv2.inRange(hsv, (20, 90, 120), (40, 255, 255))
    k = cv2.getStructuringElement(cv2.MORPH_RECT, (7, 7))
    mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, k)
    mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, k)
    n, lab, stats, cent = cv2.connectedComponentsWithStats(mask, 8)
    H, W = img.shape[:2]
    area_img = H * W
    boxes = []
    for i in range(1, n):
        x, y, w, h, a = stats[i]
        if a < 900:            # ruido / letras amarillas sueltas
            continue
        fill = a / float(w * h)
        ar = w / float(h)
        boxes.append(dict(x=int(x), y=int(y), w=int(w), h=int(h), area=int(a),
                          fill=round(fill, 2), ar=round(ar, 2),
                          pct=round(100.0 * a / area_img, 3)))
    t_proc = time.perf_counter() - t1
    return img, mask, boxes, t_read, t_proc

img, mask, boxes, t_read, t_proc = detect(P)
boxes.sort(key=lambda b: -b["area"])
print(f"imagen {img.shape[1]}x{img.shape[0]}  read={t_read*1000:.1f}ms  proc={t_proc*1000:.1f}ms")
print(f"blobs amarillos >=900px: {len(boxes)}")
for b in boxes:
    print(b)

# timing repetido (sin I/O)
hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)
ts = []
for _ in range(20):
    t = time.perf_counter()
    m = cv2.inRange(hsv, (20, 90, 120), (40, 255, 255))
    k = cv2.getStructuringElement(cv2.MORPH_RECT, (7, 7))
    m = cv2.morphologyEx(m, cv2.MORPH_CLOSE, k)
    m = cv2.morphologyEx(m, cv2.MORPH_OPEN, k)
    cv2.connectedComponentsWithStats(m, 8)
    ts.append((time.perf_counter() - t) * 1000)
print(f"pipeline puro: media {np.mean(ts):.1f}ms  min {min(ts):.1f}ms  max {max(ts):.1f}ms")
cv2.imwrite("/tmp/claude-1000/-home-anderson-Projects-startups/396da9f5-16ea-4249-b510-feab9b1909aa/scratchpad/mask.png", mask)
vis = img.copy()
for b in boxes:
    cv2.rectangle(vis, (b["x"], b["y"]), (b["x"]+b["w"], b["y"]+b["h"]), (0, 255, 0), 3)
cv2.imwrite("/tmp/claude-1000/-home-anderson-Projects-startups/396da9f5-16ea-4249-b510-feab9b1909aa/scratchpad/vis.png", vis)
