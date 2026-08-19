import cv2, numpy as np, time
G="/home/anderson/Projects/startups/tacu-project/tacu-backend/testdata/cartas/galponcito.jpeg"
img=cv2.imread(G)
def run(im):
    hsv=cv2.cvtColor(im,cv2.COLOR_BGR2HSV)
    m=cv2.inRange(hsv,(20,90,120),(40,255,255))
    k=cv2.getStructuringElement(cv2.MORPH_RECT,(7,7))
    m=cv2.morphologyEx(m,cv2.MORPH_CLOSE,k); m=cv2.morphologyEx(m,cv2.MORPH_OPEN,k)
    cnts,_=cv2.findContours(m,cv2.RETR_EXTERNAL,cv2.CHAIN_APPROX_SIMPLE)
    out=[]
    for c in cnts:
        a=cv2.contourArea(c)
        if a<900: continue
        (cx,cy),(w,h),ang=cv2.minAreaRect(c)
        if w<1 or h<1: continue
        f=a/(w*h); ar=max(w,h)/min(w,h)
        if f<0.75: continue
        if a>12000:
            if f>=0.80 and ar>1.25 and a<40000: out.append(('col',int(cx),int(cy),int(w),int(h)))
            continue
        if a>=4500 and ar<=2.2: out.append(('stk',int(cx),int(cy),int(w),int(h)))
    return out
print("minAreaRect, base:",len(run(img)))
for ang in (-12,-8,-4,0,4,8,12):
    M=cv2.getRotationMatrix2D((800,600),ang,1.0)
    print(f"  rot {ang:+d}: {len(run(cv2.warpAffine(img,M,(1600,1200))))}")
ts=[]
for _ in range(20):
    t=time.perf_counter(); run(img); ts.append((time.perf_counter()-t)*1000)
print(f"tiempo minAreaRect (incl cvtColor): {np.mean(ts):.2f}ms")
