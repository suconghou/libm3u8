# libm3u8


###  m3u8 file 


init

```
m := libm3u8.NewFromURL(nextURL)
```


for stream download
```
io.Copy(os.Stdout, m.Stream(util.GetBody))
```



### cmd

play a m3u8 url

```
./main play http://xxx | mpv -
```


print playlist by m3u8 url

```
./main list http://xxx
```

read playlist url

```
cat playlist.txt | ./main | mpv -
```
