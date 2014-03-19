package main

import (
        "os"
        "io/ioutil"
        "path/filepath"
        "log"
)

func Walk(path string) []string {

    var result []string

    stat, err := os.Lstat(path)
    if err != nil {
        log.Fatal(err)
    }

    if stat.IsDir() {
        fileList, err := ioutil.ReadDir(path)
        if err != nil {
            log.Fatal("Cant Read Directory", err)
        }
        for _, file := range fileList {
            result = append(result, Walk(filepath.Join(path, file.Name()))...)
        }

    } else {

        if ok, _ := filepath.Match("*.json", filepath.Base(path)); ok {
            result = append(result, path)
        }
    }   
    return result 
}

func ReadFile(fileName string) []byte {

         result, err := ioutil.ReadFile(fileName)
         if err != nil {
                log.Fatal(err)
         }
         return result
}