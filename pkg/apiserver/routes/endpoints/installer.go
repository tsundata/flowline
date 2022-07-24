package endpoints

import (
	"fmt"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/apiserver/routes/endpoints/handlers"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"net/http"
	"reflect"
	"sort"
)

// Struct capturing information about an action ("GET", "POST", "WATCH", "PROXY", etc).
type action struct {
	Verb   string               // Verb identifying the action ("GET", "POST", "WATCH", "PROXY", etc).
	Path   string               // The path of the action
	Params []*restful.Parameter // List of parameters associated with the action.
	UID    string
}

type APIInstaller struct {
	storages map[string]rest.Storage
}

func NewAPIInstaller(storages map[string]rest.Storage) *APIInstaller {
	return &APIInstaller{storages: storages}
}

func (a *APIInstaller) newWebService() *restful.WebService {
	return NewWebService(constant.GroupName, constant.Version)
}

func (a *APIInstaller) Install() (*restful.WebService, error) {
	ws := a.newWebService()
	paths := make([]string, len(a.storages))
	var i = 0
	for path := range a.storages {
		paths[i] = path
		i++
	}
	sort.Strings(paths)
	for _, path := range paths {
		err := a.registerResourceHandlers(path, a.storages[path], ws)
		if err != nil {
			return nil, err
		}
	}

	return ws, nil
}

func (a *APIInstaller) registerResourceHandlers(resource string, storage rest.Storage, ws *restful.WebService) error {
	tags := []string{resource}
	var rs []*restful.RouteBuilder

	scope := registry.NewRequestScope()

	var params []*restful.Parameter
	var actions []action

	resourcePath := resource
	resourceParams := params
	UID := ""

	creater, isCreater := storage.(rest.Creater)
	lister, isLister := storage.(rest.Lister)
	getter, isGetter := storage.(rest.Getter)
	deleter, isDeleter := storage.(rest.Deleter)
	collectionDeleter, isCollectionDeleter := storage.(rest.CollectionDeleter)
	updater, isUpdater := storage.(rest.Updater)
	patcher, isPatcher := storage.(rest.Patcher)
	watcher, isWatcher := storage.(rest.Watcher)
	storageMeta, isMetadata := storage.(rest.StorageMetadata)
	if !isMetadata {
		storageMeta = defaultStorageMetadata{}
	}

	actions = appendIf(actions, action{"GET", resourcePath, resourceParams, UID}, isGetter)
	actions = appendIf(actions, action{"LIST", resourcePath, resourceParams, UID}, isLister)
	actions = appendIf(actions, action{"POST", resourcePath, resourceParams, UID}, isCreater)
	actions = appendIf(actions, action{"PUT", resourcePath, resourceParams, UID}, isUpdater)
	actions = appendIf(actions, action{"DELETE", resourcePath, resourceParams, UID}, isDeleter)
	actions = appendIf(actions, action{"DELETECOLLECTION", resourcePath, resourceParams, UID}, isCollectionDeleter)
	actions = appendIf(actions, action{"PATCH", resourcePath, resourceParams, UID}, isPatcher)
	actions = appendIf(actions, action{"WATCH", resourcePath, resourceParams, UID}, isWatcher)

	for _, action := range actions {
		producedObject := storageMeta.ProducesObject(action.Verb)
		if producedObject == nil {
			producedObject = meta.Unknown{}
		}

		uidParam := ws.PathParameter("uid", "uid of the resource").DataType("string")
		switch action.Verb {
		case "GET":
			handler := handlers.GetResource(getter, scope)
			getRoute := ws.GET(resource+"/{uid}").To(handler).
				Doc(fmt.Sprintf("Get %s resource", resource)).
				Operation(resource+"Get").
				Metadata(restfulspec.KeyOpenAPITags, tags).
				Returns(http.StatusOK, "OK", producedObject).
				Writes(producedObject)
			getRoute.Param(uidParam)
			rs = append(rs, getRoute)
		case "LIST":
			handler := handlers.ListResource(lister, scope)
			listRoute := ws.GET(resource+"/list").To(handler).
				Doc(fmt.Sprintf("List %s resource", resource)).
				Operation(resource+"List").
				Metadata(restfulspec.KeyOpenAPITags, tags).
				Returns(http.StatusOK, "OK", producedObject).
				Writes(producedObject)
			rs = append(rs, listRoute)
		case "POST":
			handler := handlers.CreateResource(creater, scope)
			postRoute := ws.POST(resource).To(handler).
				Doc(fmt.Sprintf("Create %s resource", resource)).
				Operation(resource+"Create").
				Metadata(restfulspec.KeyOpenAPITags, tags).
				Returns(http.StatusOK, "OK", producedObject).
				Reads(producedObject).
				Writes(producedObject)
			rs = append(rs, postRoute)
		case "PUT":
			handler := handlers.UpdateResource(updater, scope)
			putRoute := ws.PUT(resource+"/{uid}").To(handler).
				Doc(fmt.Sprintf("Update %s resource", resource)).
				Operation(resource+"Update").
				Metadata(restfulspec.KeyOpenAPITags, tags).
				Returns(http.StatusOK, "OK", producedObject).
				Reads(producedObject).
				Writes(producedObject)
			putRoute.Param(uidParam)
			rs = append(rs, putRoute)
		case "DELETE":
			handler := handlers.DeleteResource(deleter, scope)
			deleteRoute := ws.DELETE(resource+"/{uid}").To(handler).
				Doc(fmt.Sprintf("Delete %s resource", resource)).
				Operation(resource+"Delete").
				Metadata(restfulspec.KeyOpenAPITags, tags).
				Returns(http.StatusOK, "OK", producedObject)
			deleteRoute.Param(uidParam)
			rs = append(rs, deleteRoute)
		case "DELETECOLLECTION":
			fmt.Println("DELETECOLLECTION", resource, collectionDeleter)
		case "PATCH":
			fmt.Println("PATCH", resource, patcher)
		case "WATCH":
			handler := handlers.WatchResource(watcher, scope)
			watchRoute := ws.GET(resource+"/{uid}/watch").To(handler).
				Doc(fmt.Sprintf("Watch %s resource", resource)).
				Operation(resource+"Watch").
				Metadata(restfulspec.KeyOpenAPITags, tags)
			watchRoute.Param(uidParam)
			rs = append(rs, watchRoute)

			handler = handlers.WatchListResource(watcher, scope)
			watchListRoute := ws.GET(resource+"/watch").To(handler).
				Doc(fmt.Sprintf("Watch List %s resource", resource)).
				Operation(resource+"WatchList").
				Metadata(restfulspec.KeyOpenAPITags, tags)
			rs = append(rs, watchListRoute)
		default:
			return fmt.Errorf("unrecognized action verb: %s", action.Verb)
		}
	}

	for _, route := range rs {
		ws.Route(route)
	}

	return nil
}

func appendIf(actions []action, a action, shouldAppend bool) []action {
	if shouldAppend {
		actions = append(actions, a)
	}
	return actions
}

// defaultStorageMetadata provides default answers to rest.StorageMetadata.
type defaultStorageMetadata struct{}

// defaultStorageMetadata implements rest.StorageMetadata
var _ rest.StorageMetadata = defaultStorageMetadata{}

func (defaultStorageMetadata) ProducesMIMETypes(verb string) []string {
	return nil
}

func (defaultStorageMetadata) ProducesObject(verb string) interface{} {
	return nil
}

// indirectArbitraryPointer returns *ptrToObject for an arbitrary pointer
func indirectArbitraryPointer(ptrToObject interface{}) interface{} {
	return reflect.Indirect(reflect.ValueOf(ptrToObject)).Interface()
}
