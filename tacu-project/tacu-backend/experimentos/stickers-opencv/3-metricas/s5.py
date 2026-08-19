import cv2, numpy as np, time
img=cv2.imread("/home/anderson/Projects/startups/tacu-project/tacu-backend/testdata/cartas/galponcito.jpeg")
H,W=img.shape[:2]
t=time.perf_counter()
hsv=cv2.cvtColor(img,cv2.COLOR_BGR2HSV)
# el banner es rojo saturado; el fondo/pliegue no lo es
red=cv2.inRange(hsv,(0,80,50),(12,255,255))|cv2.inRange(hsv,(168,80,50),(179,255,255))
red=cv2.morphologyEx(red,cv2.MORPH_CLOSE,cv2.getStructuringElement(cv2.MORPH_RECT,(15,15)))
col=(red>0).mean(axis=0)   # fraccion de pixeles rojos por columna
dt=(time.perf_counter()-t)*1000
# 1) recorte del banner (donde hay rojo)
on=np.where(col>0.15)[0]
print(f"banner x: {on.min()}..{on.max()} (imagen 0..{W-1})  [{dt:.1f}ms]")
# 2) minimo interior = pliegue/separacion de paneles
inner=col[on.min()+150:on.max()-150]
i=int(np.argmin(inner))+on.min()+150
print(f"minimo interior en x={i} valor={col[i]:.3f}  (media={col[on.min():on.max()].mean():.3f})")
# ancho del valle
thr=col[on.min():on.max()].mean()*0.55
valley=[x for x in range(on.min(),on.max()) if col[x]<thr]
if valley:
    import itertools
    groups=[]
    for k,g in itertools.groupby(enumerate(valley),lambda p:p[1]-p[0]):
        g=[p[1] for p in g]; groups.append((g[0],g[-1]))
    groups=[g for g in groups if g[1]-g[0]>3]
    print("valles (x0,x1):",groups)
# 3) filas: proyeccion horizontal de texto blanco/claro dentro de cada panel
for name,(x0,x1) in (("izq",(on.min(),i)),("der",(i,on.max()))):
    sub=hsv[:,x0:x1]
    light=((sub[:,:,2]>170)&(sub[:,:,1]<80)).mean(axis=1)
    rows=light>0.02
    d=np.diff(rows.astype(int)); starts=np.where(d==1)[0]; ends=np.where(d==-1)[0]
    print(f"panel {name} x{x0}-{x1}: {len(starts)} bandas de texto claro")
