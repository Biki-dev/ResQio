"use client";

import { useState } from "react";
import Navbar from "@/components/Navbar";
import HeroSection from "@/components/HeroSection";
import RegistrationModal from "@/components/RegistrationModal";
import Footer from "@/components/Footer";
import ResponsePortal from "@/components/ResponsePortal";


export default function LandingPage() {
  const [isModalOpen, setIsModalOpen] = useState(false);

  return (
    <main id="main-content" className="flex min-h-screen flex-col bg-paper text-ink">
      <Navbar onGetStartedClick={() => setIsModalOpen(true)} />
   
      <HeroSection onGetStartedClick={() => setIsModalOpen(true)} />
      <ResponsePortal />
      <Footer />

      <RegistrationModal
        open={isModalOpen}
        onClose={() => setIsModalOpen(false)}
      />
    </main>
  );
}
