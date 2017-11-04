# libm3u8


###  m3u8 file 

init

```
m, err := libm3u8.NewFromURL(url, nil)
```

for file links

```
io.Copy(os.Stdout, m)
```

for stream download
```
io.Copy(os.Stdout, m.Play())
```


### url links

```
r := libm3u8.NewReader(scanner *bufio.Scanner)
```

get stream data by url list
