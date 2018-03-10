# libm3u8


###  m3u8 file 

init

```
m := libm3u8.NewFromURL(nextURL)
```

for file links

```
io.Copy(os.Stdout, m.PlayList())
```

for stream download
```
io.Copy(os.Stdout, m.Play())
```

for origin url
```
io.Copy(os.Stdout, m)
```



### url links

```
r := libm3u8.NewReader(scanner *bufio.Scanner)
```

get stream data by url list
