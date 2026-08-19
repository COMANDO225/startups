import cv2, numpy as np
img=cv2.imread("/home/anderson/Projects/startups/tacu-project/tacu-backend/testdata/cartas/galponcito.jpeg")
g=cv2.cvtColor(img,cv2.COLOR_BGR2GRAY).astype(np.float32)
band=g[100:1100,:]
dark=band.mean(axis=0)            # oscuridad por columna
sob=np.abs(cv2.Sobel(band,cv2.CV_32F,1,0,ksize=3)).mean(axis=0)  # energia de borde vertical
for name,sig,pick in (("oscuridad",dark,np.argmin),("borde-vert",sob,np.argmax)):
    s=cv2.GaussianBlur(sig.reshape(1,-1),(31,1),0).ravel()
    seg=s[400:1200]
    x=int(pick(seg))+400
    print(f"{name}: extremo interior en x={x}  val={s[x]:.1f}  media={s[400:1200].mean():.1f}")
    # top-5 candidatos separados
    order=np.argsort(seg if pick is np.argmin else -seg)+400
    sel=[]
    for c in order:
        if all(abs(c-p)>60 for p in sel): sel.append(int(c))
        if len(sel)==5: break
    print("   top5:",sel)
print("pliegue real (medido a ojo en la foto): x~=805")
