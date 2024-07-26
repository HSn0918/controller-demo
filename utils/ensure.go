package utils

import (
	"container/list"
	"reflect"
)

// EnsureMapFieldsInitializedBFS 使用广度优先搜索初始化结构体中的所有 map 字段
func EnsureMapFieldsInitializedBFS(obj interface{}) {
	queue := list.New()
	queue.PushBack(reflect.ValueOf(obj))

	for queue.Len() > 0 {
		elem := queue.Front()
		v := elem.Value.(reflect.Value)
		queue.Remove(elem)

		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		if v.Kind() != reflect.Struct {
			continue
		}

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldType := field.Type()

			// 检查并初始化 map 字段
			if field.Kind() == reflect.Map && field.IsNil() {
				field.Set(reflect.MakeMap(fieldType))
			}

			// 如果字段是结构体或指向结构体的指针，添加到队列中
			if (field.Kind() == reflect.Struct) || (field.Kind() == reflect.Ptr && field.Elem().Kind() == reflect.Struct) {
				queue.PushBack(field)
			}
		}
	}
}
