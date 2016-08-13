
package main

import "fmt"
import "time"
// import "sync"

type RetInfo struct {

    ret interface{}  
        //~ And it can be anything.
        //~ This is the return value.

    cb  interface{}  
        //~ And these are the allowed prototypes for callback
        //  func()
        //~ func(interface{})
        //~ func(s[]interface{})
        //~ 
}

type CallInfo struct {
    sig  interface{} 
    args []interface{}    //~ It might be a []interface{}
    retInfo chan *RetInfo
    cb interface{}  //~ The callback
}

type Server struct {
    _funcs map[interface{}]interface{}  //` Any thing to func
    _callInfo chan *CallInfo
}

type Client struct {
    _host   *Server
    syncRet  chan *RetInfo
    asyncRet chan *RetInfo  //~ 若是没有初始化(通过make, 那么也不会发生错误...你可以往里面读)
    _pending int
}

func CreateServer(length int)*Server{
    svr := new(Server)
    svr._funcs = make(map[interface{}]interface{}) 
        //~ And the make is done.
    svr._callInfo = make(chan *CallInfo, length)
    return svr
}

func (svr *Server) Register(name interface{}, proc interface{}){
    //~ Checking of the type
    //~ Only the specific ones are allowed.
    // fmt.Printf("This is <%v>\n", proc)
    switch proc.(type) {
    case func([]interface{}):
    case func([]interface{})interface{}:
    case func([]interface{})[]interface{}:
    default:
        fmt.Println("Not the type we need :<%v>", name)
        return
    }
    svr._funcs[name] = proc //~ And the map is done.
}

func (svr *Server) StartServe() {
    go func() {
        for{
            time.Sleep(time.Second)
            svr.exec( <- svr._callInfo)
        }
        //~ Yes. Here I am
    }()
}

//////////////////////////////////////////////////
//////////////// (Not the master thread)
//~ in the goroutine.
func (svr *Server) exec(ci *CallInfo) {
    run, ok := svr._funcs[ci.sig]  //~ Fetching the function by ref(such as strings or pointers)
    if !ok {
        fmt.Printf("Not function for <%v>\n", ci.sig)
        return
    }
    // fmt.Println("Another request for RPC")
    //~ 这里是否需要看一下run的类型? 也就是签名. (我们总是自带类型的. 通过()转换判空判定)
    //~ Phase Dos:
    switch run.(type) {
    case func([]interface{}):
        // fmt.Println("call0 - exec")
        run.(func([]interface{}))(ci.args)
        ci.retInfo <- &RetInfo{
            cb:ci.cb,
        }   //~ Just return. Notification to client. 
    case func([]interface{})interface{}:
        // fmt.Println("call1 - exec")
        ret:= run.(func([]interface{})interface{})(ci.args)
        ci.retInfo <- &RetInfo{
            ret:ret,        //~ interface{} is interface{}
            cb:ci.cb,
        }
    case func([]interface{})[]interface{}:
        // fmt.Println("callN - exec")
        ret:= run.(func([]interface{})[]interface{})(ci.args)
        ci.retInfo <- &RetInfo{
            ret:ret,
            cb:ci.cb,
        }
    }
    fmt.Println("<done with one.>")
}

//~ The client object
func CreateClient(svr *Server) *Client {
    clt := new(Client)
    clt._host = svr
    clt.syncRet = make(chan *RetInfo, 1)
    clt.asyncRet= make(chan *RetInfo, 1)
    return clt
}

func (clt *Client)IsIdle()bool{
    return clt._pending == 0
}
func (clt *Client)addAC(){
    clt._pending ++
}
func (clt *Client)decAC(){
    clt._pending --
}

func (clt *Client) Call0 (sig interface{}, args...interface{}) {
    clt._host._callInfo <- &CallInfo {
        sig:sig,
        args:args, //~ This is the grammar you need.
        retInfo:clt.syncRet,
    }
    <- clt.syncRet //~ Make it sync
}

func (clt *Client) Call1 (sig interface{}, args...interface{})interface{}{
    //~ 初始化完了才甩出指针.
    clt._host._callInfo <- &CallInfo {
        sig:sig,
        args:args,   //~ This is the grammar you need.
        retInfo:clt.syncRet,
    }
    ret := <- clt.syncRet
    return ret.ret
}

///////////////////
//~ And by the constraint of the calling convention
func (clt *Client) AsyncCall(sig interface{}, args[] interface{}, cb interface{}, async2 bool) {
    switch cb.(type){
    case func():
    case func(interface{}):
    case func([]interface{}):
    default:
        fmt.Println("callback prototype error")
        return
    }
    host := clt._host
    //~ Pack up the call-info
    //~ By channelling switching

    host._callInfo <- &CallInfo{
        sig:sig, //~ Whatever it is, as long as it is unique in this process
        args:args,
        retInfo:clt.asyncRet, //~ And this is the async return
        cb:cb, //~ Always tells the callback
        //cb:cb,
    }
    fmt.Println("Async call is made(and the async-ret is passed to server)")
    //~ Do the callback: in another goroutine ??
    if async2 {go func(){
        // fmt.Println("Starting select ?")
            //~ No matter 
            //~ it make by 1 
            //~ or make without the second parameter(which means it is in blocking mode)
            //~ or make by 0(essentially the same as one above)
            //~ We always blocked in this `select'. If there is only one clause.
            //~ If however, `default' is added to `cases'
            //~ The try-out:
            //~ Case Uno: asyncRet 为make(),  没有default: 阻塞
            //~ Case Dos: asyncRet 为make(),  有default:  不阻塞. 跳过了.
            //~ Case Tres:asyncRet 为make(1), 没有default: 阻塞
            //~ Case Cuatro:asyncRet为make(1),有default:  不阻塞
            //~ 所以, default改变了select的行为.(select可以让buffered chan阻塞, 必要条件是没有default)
        select {
        case ri := <- clt.asyncRet:
            switch cb.(type){
            case func():
                cb.(func())()
            case func(interface{}):
                cb.(func(interface{}))(ri.ret)
            case func([]interface{}):
                cb.(func([]interface{}))(ri.ret.([]interface{}))
            }
        // default:
        //     fmt.Println("This is <default> (Which means unblocking happens)")
        }
        // fmt.Println("Callback select ends ??")
    }()} else {
        clt.addAC()
    }
}    

//~ In the client's thread.
func (clt *Client) Poll(){
    if clt.IsIdle() {
        //fmt.Println("Already idle")
        return //~ Nothing to do
    }
    //fmt.Println("Now pending:", clt._pending)
    select {
    case ri:= <- clt.asyncRet:
        clt.decAC()
        // fmt.Println("Get from async-ret")
        switch ri.cb.(type) {
        case func():
            ri.cb.(func())()
        case func(interface{}):
            ri.cb.(func(interface{}))(ri.ret)
        case func([]interface{}):
            ri.cb.(func([]interface{}))(ri.ret.([]interface{}))
        default:
            panic("No for cb >???")
        }
        //~ No fall through
    default:
        fmt.Println("Missing(polling)")
    }
}


func doWait(i time.Duration){
    fmt.Printf("Wait for <%v> second(s)\n", i)
    time.Sleep(time.Second * i)
}

func main() {
    // var wg sync.WaitGroup
    //wg.Add(1)
    svr := CreateServer(1) //~ Make it async.
    svr.Register("f0", func(args[]interface{}){
        fmt.Printf("This is args for f0:<%v>\n", args)
        fmt.Println("This is f0 called")
    })
    svr.Register("add", func(args[]interface{})interface{}{
        //~ fmt.Println("The len of args = ", len(args)) 
        //~ This varies from time to time.
        a := args[0].(int)
        b := args[1].(int) //This may properly not be the one you expected.
        //fmt.Println("Preview of result:", a + b)
        return a + b
    })
    svr.StartServe()
    //wg.Wait()
    clt := CreateClient(svr)
    // clt.Call0("f0", 3,2,3,4,4,4)
    // fmt.Println("Yes")
    // res := clt.Call1("add", 3, 5)
    // fmt.Printf("Sync-call result is <%v>\n", res)

    // //~ Async Test Uno
    // fmt.Println("Starting the async call test")
    // value := 255
    // clt.AsyncCall("add", []interface{}{2, 3}, func(res interface{}){
    //     result := res.(int)
    //     fmt.Printf("(%v)The result is <%v> (async)(in another GR)\n", value, result)
    // }, true)
    // for{
    //     time.Sleep(time.Second)
    // }

    // Async Test Dos
    clt.AsyncCall("add", []interface{}{3, 5}, func(res interface{}){
        result := res.(int)
        fmt.Printf("Result is <%v> for async-call\n", result)
    }, false)
    for {
        clt.Poll()
        time.Sleep(time.Millisecond * 333)
        // time.Sleep(time.Millisecond * 333)  / And it works
    }
    // doWait(0)
}