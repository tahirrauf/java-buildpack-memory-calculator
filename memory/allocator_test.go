// Encoding: utf-8
// Cloud Foundry Java Buildpack
// Copyright (c) 2015-2017 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package memory_test

import (
	"github.com/cloudfoundry/java-buildpack-memory-calculator/memory"
	"github.com/cloudfoundry/java-buildpack-memory-calculator/memory/vmoptionsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Allocator", func() {

	const (
		testMemSizeString   = "500M"
		testMemOptionString = "22M"
	)

	var (
		testMemSize       memory.MemSize
		testMemOptionSize memory.MemSize
		vmOptions         *vmoptionsfakes.FakeVmOptions
		allocator         memory.Allocator
		poolType          string
		err               error
	)

	BeforeEach(func() {
		testMemSize, err = memory.NewMemSizeFromString(testMemSizeString)
		Ω(err).ShouldNot(HaveOccurred())
		testMemOptionSize, err = memory.NewMemSizeFromString(testMemOptionString)
		Ω(err).ShouldNot(HaveOccurred())
		vmOptions = &vmoptionsfakes.FakeVmOptions{}
		poolType = "metaspace"
	})

	JustBeforeEach(func() {
		allocator, err = memory.NewAllocator(poolType, vmOptions)
		Ω(err).ShouldNot(HaveOccurred())
	})

	Describe("String", func() {
		var (
			options map[memory.MemoryType]memory.MemSize
		)

		BeforeEach(func() {
			options = map[memory.MemoryType]memory.MemSize{}
			vmOptions.MemOptStub = func(memoryType memory.MemoryType) memory.MemSize {
				return options[memoryType]
			}

			vmOptions.SetMemOptStub = func(memoryType memory.MemoryType, size memory.MemSize) {
				options[memoryType] = size
			}

			vmOptions.DeltaStringStub = func() string {
				return "some string representation"
			}
		})

		It("should delegate to the DeltaString method of the embedded VmOptions", func() {
			Ω(allocator.String()).Should(Equal("some string representation"))
		})
	})

	Describe("memory size calculations", func() {
		Context("when poolType is metaspace", func() {
			var (
				stackThreads int

				expectedMaxMetaspaceSize         memory.MemSize
				expectedReservedCodeCacheSize    memory.MemSize
				expectedMaxDirectMemorySize      memory.MemSize

				options map[memory.MemoryType]memory.MemSize

				vmOptionsCopy *vmoptionsfakes.FakeVmOptions
			)

			BeforeEach(func() {
				stackThreads = 10

				expectedMaxMetaspaceSize = memory.NewMemSize(19800000)
				expectedReservedCodeCacheSize = memory.NewMemSize(240 * 1024 * 1024)
				expectedMaxDirectMemorySize = memory.NewMemSize(10 * 1024 * 1024)

				options = map[memory.MemoryType]memory.MemSize{}
				vmOptions.MemOptStub = func(memoryType memory.MemoryType) memory.MemSize {
					return options[memoryType]
				}

				vmOptions.SetMemOptStub = func(memoryType memory.MemoryType, size memory.MemSize) {
					options[memoryType] = size
				}

				vmOptionsCopy = &vmoptionsfakes.FakeVmOptions{}

				vmOptionsCopy.StringReturns("vmoptions-copy-output")

				vmOptions.CopyStub = func() memory.VmOptions {
					return vmOptionsCopy
				}
			})

			JustBeforeEach(func() {
				err = allocator.Calculate(1000, stackThreads, testMemSize)
			})

			Describe("maximum metaspace size", func() {
				It("should produce the correct estimate", func() {
					Ω(options[memory.MaxMetaspaceSize]).Should(Equal(expectedMaxMetaspaceSize))
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("when the value has been set", func() {
					BeforeEach(func() {
						options[memory.MaxMetaspaceSize] = testMemOptionSize
					})

					It("should preserve the set value", func() {
						Ω(options[memory.MaxMetaspaceSize]).Should(Equal(testMemOptionSize))
						Ω(err).ShouldNot(HaveOccurred())
					})
				})
			})

			Describe("reserved code cache size", func() {
				It("should produce the correct default", func() {
					Ω(options[memory.ReservedCodeCacheSize]).Should(Equal(expectedReservedCodeCacheSize))
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("when the value has been set", func() {
					BeforeEach(func() {
						options[memory.ReservedCodeCacheSize] = testMemOptionSize
					})

					It("should preserve the set value", func() {
						Ω(options[memory.ReservedCodeCacheSize]).Should(Equal(testMemOptionSize))
						Ω(err).ShouldNot(HaveOccurred())
					})
				})
			})

			Describe("maximum direct memory size", func() {
				It("should produce the correct estimate", func() {
					Ω(options[memory.MaxDirectMemorySize]).Should(Equal(expectedMaxDirectMemorySize))
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("when the value has been set", func() {
					BeforeEach(func() {
						options[memory.MaxDirectMemorySize] = testMemOptionSize
					})

					It("should preserve the set value", func() {
						Ω(options[memory.MaxDirectMemorySize]).Should(Equal(testMemOptionSize))
						Ω(err).ShouldNot(HaveOccurred())
					})
				})
			})

			Describe("maximum heap size", func() {
				var (
					expectedStackSpace      memory.MemSize
					expectedAllocatedMemory memory.MemSize
				)

				BeforeEach(func() {
					expectedStackSpace = memory.NewMemSize(1024 * 1024).Scale(float64(stackThreads))
					expectedAllocatedMemory = expectedMaxMetaspaceSize.Add(expectedMaxDirectMemorySize).Add(expectedReservedCodeCacheSize).
						Add(expectedStackSpace)
				})

				It("should produce the correct estimate", func() {
					Ω(options[memory.MaxHeapSize]).Should(Equal(testMemSize.Subtract(expectedAllocatedMemory)))
				})

				It("should set the stack size to a default value", func() {
					Ω(options[memory.StackSize]).Should(Equal(memory.NewMemSize(1024 * 1024)))
				})

				Context("when the stack size has been specified", func() {
					BeforeEach(func() {
						options[memory.StackSize] = memory.NewMemSize(2 * 1024 * 1024) // double the default value
					})

					It("should produce the correct estimate", func() {
						Ω(options[memory.MaxHeapSize]).Should(Equal(testMemSize.Subtract(expectedAllocatedMemory.Add(expectedStackSpace))))
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("should preserve the specified stack size", func() {
						Ω(options[memory.StackSize]).Should(Equal(memory.NewMemSize(2 * 1024 * 1024)))
					})
				})

				Context("when there is insufficient memory remaining", func() {
					BeforeEach(func() {
						options[memory.MaxDirectMemorySize] = memory.NewMemSize(500 * 1024 * 1024)
						vmOptions.StringReturns("vmoptions-output")
					})

					It("should return an error", func() {
						Ω(err).Should(MatchError("There is insufficient memory remaining for heap. Memory limit 500M is less than allocated memory 787335K (vmoptions-copy-output, -Xss1M * 10 threads)"))
					})

				})

				Context("when the value has been set", func() {
					BeforeEach(func() {
						options[memory.MaxHeapSize] = testMemOptionSize
					})

					It("should preserve the set value", func() {
						Ω(options[memory.MaxHeapSize]).Should(Equal(testMemOptionSize))
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("should not set the stack size", func() {
						_, ok := options[memory.StackSize]
						Ω(ok).Should(BeFalse())
					})
				})
			})
		})

		Context("when poolType is permgen", func() {
			var (
				stackThreads int

				expectedMaxPermSize           memory.MemSize
				expectedReservedCodeCacheSize memory.MemSize
				expectedMaxDirectMemorySize   memory.MemSize

				options map[memory.MemoryType]memory.MemSize

				vmOptionsCopy *vmoptionsfakes.FakeVmOptions
			)

			BeforeEach(func() {
				poolType = "permgen"
				stackThreads = 10

				expectedMaxPermSize = memory.NewMemSize(13000000)
				expectedReservedCodeCacheSize = memory.NewMemSize(48 * 1024 * 1024)
				expectedMaxDirectMemorySize = memory.NewMemSize(10 * 1024 * 1024)

				options = map[memory.MemoryType]memory.MemSize{}
				vmOptions.MemOptStub = func(memoryType memory.MemoryType) memory.MemSize {
					return options[memoryType]
				}

				vmOptions.SetMemOptStub = func(memoryType memory.MemoryType, size memory.MemSize) {
					options[memoryType] = size
				}

				vmOptionsCopy = &vmoptionsfakes.FakeVmOptions{}

				vmOptionsCopy.StringReturns("vmoptions-copy-output")

				vmOptions.CopyStub = func() memory.VmOptions {
					return vmOptionsCopy
				}
			})

			JustBeforeEach(func() {
				err = allocator.Calculate(1000, stackThreads, testMemSize)
			})

			Describe("maximum metaspace size", func() {
				It("should produce the correct estimate", func() {
					Ω(options[memory.MaxPermSize]).Should(Equal(expectedMaxPermSize))
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("when the value has been set", func() {
					BeforeEach(func() {
						options[memory.MaxPermSize] = testMemOptionSize
					})

					It("should preserve the set value", func() {
						Ω(options[memory.MaxPermSize]).Should(Equal(testMemOptionSize))
						Ω(err).ShouldNot(HaveOccurred())
					})
				})
			})

			Describe("reserved code cache size", func() {
				It("should produce the correct default", func() {
					Ω(options[memory.ReservedCodeCacheSize]).Should(Equal(expectedReservedCodeCacheSize))
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("when the value has been set", func() {
					BeforeEach(func() {
						options[memory.ReservedCodeCacheSize] = testMemOptionSize
					})

					It("should preserve the set value", func() {
						Ω(options[memory.ReservedCodeCacheSize]).Should(Equal(testMemOptionSize))
						Ω(err).ShouldNot(HaveOccurred())
					})
				})
			})

			Describe("maximum direct memory size", func() {
				It("should produce the correct estimate", func() {
					Ω(options[memory.MaxDirectMemorySize]).Should(Equal(expectedMaxDirectMemorySize))
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("when the value has been set", func() {
					BeforeEach(func() {
						options[memory.MaxDirectMemorySize] = testMemOptionSize
					})

					It("should preserve the set value", func() {
						Ω(options[memory.MaxDirectMemorySize]).Should(Equal(testMemOptionSize))
						Ω(err).ShouldNot(HaveOccurred())
					})
				})
			})

			Describe("maximum heap size", func() {
				var (
					expectedStackSpace      memory.MemSize
					expectedAllocatedMemory memory.MemSize
				)

				BeforeEach(func() {
					expectedStackSpace = memory.NewMemSize(1024 * 1024).Scale(float64(stackThreads))
					expectedAllocatedMemory = expectedMaxPermSize.Add(expectedMaxDirectMemorySize).Add(expectedReservedCodeCacheSize).Add(expectedStackSpace)
				})

				It("should produce the correct estimate", func() {
					Ω(options[memory.MaxHeapSize]).Should(Equal(testMemSize.Subtract(expectedAllocatedMemory)))
				})

				It("should set the stack size to a default value", func() {
					Ω(options[memory.StackSize]).Should(Equal(memory.NewMemSize(1024 * 1024)))
				})

				Context("when the stack size has been specified", func() {
					BeforeEach(func() {
						options[memory.StackSize] = memory.NewMemSize(2 * 1024 * 1024) // double the default value
					})

					It("should produce the correct estimate", func() {
						Ω(options[memory.MaxHeapSize]).Should(Equal(testMemSize.Subtract(expectedAllocatedMemory.Add(expectedStackSpace))))
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("should preserve the specified stack size", func() {
						Ω(options[memory.StackSize]).Should(Equal(memory.NewMemSize(2 * 1024 * 1024)))
					})
				})

				Context("when there is insufficient memory remaining", func() {
					BeforeEach(func() {
						options[memory.MaxDirectMemorySize] = memory.NewMemSize(500 * 1024 * 1024)
						vmOptions.StringReturns("vmoptions-output")
					})

					It("should return an error", func() {
						Ω(err).Should(MatchError("There is insufficient memory remaining for heap. Memory limit 500M is less than allocated memory 584087K (vmoptions-copy-output, -Xss1M * 10 threads)"))
					})

				})

				Context("when the value has been set", func() {
					BeforeEach(func() {
						options[memory.MaxHeapSize] = testMemOptionSize
					})

					It("should preserve the set value", func() {
						Ω(options[memory.MaxHeapSize]).Should(Equal(testMemOptionSize))
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("should not set the stack size", func() {
						_, ok := options[memory.StackSize]
						Ω(ok).Should(BeFalse())
					})
				})
			})
		})
	})
})
