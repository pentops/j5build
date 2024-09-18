package j5convert

import (
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/sourcewalk"
)

func convertFile(ww *walkContext, src *sourcedef_j5pb.SourceFile) error {
	file := sourcewalk.NewRoot(src)
	return file.RangeRootElements(sourcewalk.FileCallbacks{
		SchemaCallbacks: sourcewalk.SchemaCallbacks{
			Object: func(on *sourcewalk.ObjectNode) error {
				convertObject(ww, on)
				return nil
			},
			Oneof: func(on *sourcewalk.OneofNode) error {
				convertOneof(ww, on)
				return nil
			},
			Enum: func(en *sourcewalk.EnumNode) error {
				convertEnum(ww, en)
				return nil
			},
		},
		TopicFile: func(tn *sourcewalk.TopicFileNode) error {
			subWalk := ww.subPackageFile("topic")
			return convertTopic(subWalk, tn)
		},
		ServiceFile: func(sn *sourcewalk.ServiceFileNode) error {
			subWalk := ww.subPackageFile("service")
			return convertService(subWalk, sn)
		},
	})
}

func walkerSchemaVisitor(ww *walkContext) sourcewalk.SchemaVisitor {
	return &sourcewalk.SchemaCallbacks{
		Object: func(on *sourcewalk.ObjectNode) error {
			convertObject(ww, on)
			return nil
		},
		Oneof: func(on *sourcewalk.OneofNode) error {
			convertOneof(ww, on)
			return nil
		},
		Enum: func(en *sourcewalk.EnumNode) error {
			convertEnum(ww, en)
			return nil
		},
	}
}
