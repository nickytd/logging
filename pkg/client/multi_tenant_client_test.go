// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client_test

import (
	"os"
	"time"

	valitailclient "github.com/credativ/vali/pkg/valitail/client"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"
	"github.com/weaveworks/common/logging"

	"github.com/gardener/logging/pkg/client"
	"github.com/gardener/logging/pkg/config"
)

var _ = g.Describe("Multi Tenant Client", func() {
	var (
		fakeClient *client.FakeValiClient
		mtc        client.ValiClient
	)

	g.BeforeEach(func() {
		var err error
		fakeClient = &client.FakeValiClient{}
		var infoLogLevel logging.Level
		_ = infoLogLevel.Set("info")

		mtc, err = client.NewMultiTenantClientDecorator(config.Config{},
			func(_ config.Config, _ log.Logger) (client.ValiClient, error) {
				return fakeClient, nil
			},
			level.NewFilter(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), infoLogLevel.Gokit))
		Expect(err).ToNot(HaveOccurred())
		Expect(mtc).NotTo(BeNil())
	})

	type handleArgs struct {
		ls            model.LabelSet
		t             time.Time
		s             string
		wantedTenants []model.LabelValue
	}
	g.DescribeTable("#Handle", func(args handleArgs) {
		err := mtc.Handle(args.ls, args.t, args.s)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(fakeClient.Entries) > 0).To(BeTrue())
		var gotTenants []model.LabelValue
		for _, entry := range fakeClient.Entries {
			// __gardener_multitenant_id__ should be removed after Handle() call
			_, ok := entry.Labels[client.MultiTenantClientLabel]
			Expect(ok).To(BeFalse())
			// Each tenant in the MultiTenantClientLabel should be transferred to __tenant_id__
			if tenant, ok := entry.Labels[valitailclient.ReservedLabelTenantID]; ok {
				gotTenants = append(gotTenants, tenant)
			}
		}
		// Check if all multitenants are parsed and used for forwarding to the wrapped Handle func
		Expect(len(args.wantedTenants)).To(Equal(len(gotTenants)))
		for _, tenant := range args.wantedTenants {
			Expect(tenant).To(BeElementOf(gotTenants))
		}
		// Sanity check if the timestamp and the messages re the same
		for _, entry := range fakeClient.Entries {
			Expect(entry.Timestamp).To(Equal(args.t))
			Expect(entry.Line).To(Equal(args.s))
		}
	},
		g.Entry("Handle record without reserved labels", handleArgs{
			ls:            model.LabelSet{"hostname": "test"},
			t:             time.Now(),
			s:             "test1",
			wantedTenants: []model.LabelValue{},
		}),
		g.Entry("Handle record without __gardener_multitenant_id__ reserved label", handleArgs{
			ls:            model.LabelSet{"hostname": "test", "__tenant_id__": "user"},
			t:             time.Now(),
			s:             "test1",
			wantedTenants: []model.LabelValue{"user"},
		}),
		g.Entry("Handle record with __gardener_multitenant_id__ reserved label. Separator \"; \"", handleArgs{
			ls:            model.LabelSet{"hostname": "test", "__gardener_multitenant_id__": "operator; user"},
			t:             time.Now(),
			s:             "test1",
			wantedTenants: []model.LabelValue{"operator", "user"},
		}),
		g.Entry("Handle record with __gardener_multitenant_id__ reserved label. Separator \";\"", handleArgs{
			ls:            model.LabelSet{"hostname": "test", "__gardener_multitenant_id__": "operator;user"},
			t:             time.Now(),
			s:             "test1",
			wantedTenants: []model.LabelValue{"operator", "user"},
		}),
		g.Entry("Handle record with __gardener_multitenant_id__ reserved label. Separator \" ; \" and leading and trailing spaces", handleArgs{
			ls:            model.LabelSet{"hostname": "test", "__gardener_multitenant_id__": "  operator ; user  "},
			t:             time.Now(),
			s:             "test1",
			wantedTenants: []model.LabelValue{"operator", "user"},
		}),
		g.Entry("Handle record with __gardener_multitenant_id__ reserved label with one empty. Separator \" ; \" and leading and trailing spaces.", handleArgs{
			ls:            model.LabelSet{"hostname": "test", "__gardener_multitenant_id__": "  operator ; ; user  "},
			t:             time.Now(),
			s:             "test1",
			wantedTenants: []model.LabelValue{"operator", "user"},
		}),
		g.Entry("Handle record with __gardener_multitenant_id__ and __tenant_id__ reserved labels.", handleArgs{
			ls:            model.LabelSet{"hostname": "test", "__tenant_id__": "pinokio", "__gardener_multitenant_id__": "operator; user"},
			t:             time.Now(),
			s:             "test1",
			wantedTenants: []model.LabelValue{"operator", "user"},
		}),
	)

	g.Describe("#Stop", func() {
		g.It("should stop", func() {
			Expect(fakeClient.IsGracefullyStopped).To(BeFalse())
			Expect(fakeClient.IsStopped).To(BeFalse())
			mtc.Stop()
			Expect(fakeClient.IsGracefullyStopped).To(BeFalse())
			Expect(fakeClient.IsStopped).To(BeTrue())
		})
	})

	g.Describe("#StopWait", func() {
		g.It("should stop", func() {
			Expect(fakeClient.IsGracefullyStopped).To(BeFalse())
			Expect(fakeClient.IsStopped).To(BeFalse())
			mtc.StopWait()
			Expect(fakeClient.IsGracefullyStopped).To(BeTrue())
			Expect(fakeClient.IsStopped).To(BeFalse())
		})
	})

})

var _ = g.Describe("Remove Multi Tenant Client", func() {
	var (
		fakeClient *client.FakeValiClient
		mtc        client.ValiClient
	)

	g.BeforeEach(func() {
		var err error
		fakeClient = &client.FakeValiClient{}
		var infoLogLevel logging.Level
		_ = infoLogLevel.Set("info")

		mtc, err = client.NewRemoveMultiTenantIdClientDecorator(config.Config{},
			func(_ config.Config, _ log.Logger) (client.ValiClient, error) {
				return fakeClient, nil
			},
			level.NewFilter(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), infoLogLevel.Gokit))
		Expect(err).ToNot(HaveOccurred())
		Expect(mtc).NotTo(BeNil())
	})

	type handleArgs struct {
		ls             model.LabelSet
		t              time.Time
		s              string
		wantedLabelSet model.LabelSet
	}
	g.DescribeTable("#Handle", func(args handleArgs) {
		err := mtc.Handle(args.ls, args.t, args.s)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(fakeClient.Entries) > 0).To(BeTrue())

		for _, entry := range fakeClient.Entries {
			Expect(entry.Labels).To(Equal(args.wantedLabelSet))
		}
	},
		g.Entry("Handle record without __gardener_multitenant_id__ labels.", handleArgs{
			ls:             model.LabelSet{"hostname": "test"},
			t:              time.Now(),
			s:              "test1",
			wantedLabelSet: model.LabelSet{"hostname": "test"},
		}),
		g.Entry("Handle record without __gardener_multitenant_id__ label and with __tenant_id__ label.", handleArgs{
			ls:             model.LabelSet{"hostname": "test", "__tenant_id__": "user"},
			t:              time.Now(),
			s:              "test1",
			wantedLabelSet: model.LabelSet{"hostname": "test", "__tenant_id__": "user"},
		}),
		g.Entry("Handle record with __gardener_multitenant_id__ reserved label.", handleArgs{
			ls:             model.LabelSet{"hostname": "test", "__gardener_multitenant_id__": "operator; user"},
			t:              time.Now(),
			s:              "test1",
			wantedLabelSet: model.LabelSet{"hostname": "test"},
		}),
		g.Entry("Handle record with __gardener_multitenant_id__ reserved label and __tenant_id__ label.", handleArgs{
			ls:             model.LabelSet{"hostname": "test", "__gardener_multitenant_id__": "operator;user", "__tenant_id__": "user"},
			t:              time.Now(),
			s:              "test1",
			wantedLabelSet: model.LabelSet{"hostname": "test", "__tenant_id__": "user"},
		}),
		g.Entry("Handle record without labels", handleArgs{
			ls:             model.LabelSet{},
			t:              time.Now(),
			s:              "test1",
			wantedLabelSet: model.LabelSet{},
		}),
		g.Entry("Handle record with nil label set", handleArgs{
			ls:             nil,
			t:              time.Now(),
			s:              "test1",
			wantedLabelSet: model.LabelSet{},
		}),
	)

	g.Describe("#Stop", func() {
		g.It("should stop", func() {
			Expect(fakeClient.IsGracefullyStopped).To(BeFalse())
			Expect(fakeClient.IsStopped).To(BeFalse())
			mtc.Stop()
			Expect(fakeClient.IsGracefullyStopped).To(BeFalse())
			Expect(fakeClient.IsStopped).To(BeTrue())
		})
	})

	g.Describe("#StopWait", func() {
		g.It("should stop", func() {
			Expect(fakeClient.IsGracefullyStopped).To(BeFalse())
			Expect(fakeClient.IsStopped).To(BeFalse())
			mtc.StopWait()
			Expect(fakeClient.IsGracefullyStopped).To(BeTrue())
			Expect(fakeClient.IsStopped).To(BeFalse())
		})
	})

})
